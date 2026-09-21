package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"

	"github.com/brogli/nebegit/internal/core"
)

const (
	warmPings = 5
	spikeRowN = 5000
	eventWait = 5 * time.Second
	exitWait  = 5 * time.Second
	eventRPC  = "event"
	pingRPC   = "ping"
	rowsRPC   = "rows"
)

// dialConfig is where dial reaches the server (Design): a local child
// process for the same-host baseline, or, on Windows, a child spawned
// through wsl.exe.
type dialConfig struct {
	local  string // path to a wslspike binary, run locally as "serve"
	distro string // WSL distro name
	remote string // server binary's path inside the distro
}

func (cfg dialConfig) command() (*exec.Cmd, error) {
	if cfg.distro != "" {
		return distroCommand(cfg.distro, cfg.remote)
	}
	return exec.Command(cfg.local, "serve"), nil
}

// dialReport is cases 2 and 3's measurements (Design): connect and warm-ping
// timing, the pushed event, the rows(5000) throughput, and the child's exit.
type dialReport struct {
	connect      time.Duration
	warmPingAvg  time.Duration
	eventAt      time.Duration
	eventPayload json.RawMessage
	rowsDur      time.Duration
	rowsBytes    int
	exitCode     int
	exitWithin   time.Duration
}

// runDial connects, drives cases 2 and 3, and writes the report (Design) to
// out.
func runDial(ctx context.Context, cfg dialConfig, out io.Writer) (dialReport, error) {
	cmd, err := cfg.command()
	if err != nil {
		return dialReport{}, err
	}

	client, err := startClient(cmd)
	if err != nil {
		return dialReport{}, err
	}
	defer client.close()

	var r dialReport
	start := time.Now()
	if err := client.ping(ctx); err != nil {
		return r, fmt.Errorf("connect ping: %w", err)
	}
	r.connect = time.Since(start)

	if r.warmPingAvg, err = client.warmPing(ctx); err != nil {
		return r, err
	}

	if r.eventAt, r.eventPayload, err = client.awaitEvent(start); err != nil {
		return r, err
	}

	if r.rowsDur, r.rowsBytes, err = client.fetchRows(ctx); err != nil {
		return r, err
	}

	r.exitCode, r.exitWithin, err = closeAndWait(client, cmd)
	if err != nil {
		return r, err
	}

	r.writeTo(out)
	return r, nil
}

func (r dialReport) writeTo(out io.Writer) {
	_, _ = fmt.Fprintf(out, "connect: %s (first successful ping; may include WSL boot time, excluded per QA-PERF-01)\n", r.connect)
	_, _ = fmt.Fprintf(out, "warm ping (avg of %d): %s\n", warmPings, r.warmPingAvg)
	_, _ = fmt.Fprintf(out, "event: received after %s from connect, payload %s\n", r.eventAt, r.eventPayload)
	bps := float64(r.rowsBytes) / r.rowsDur.Seconds()
	_, _ = fmt.Fprintf(out, "rows(%d): %s, %d bytes, %.0f bytes/sec\n", spikeRowN, r.rowsDur, r.rowsBytes, bps)
	_, _ = fmt.Fprintf(out, "wsl.exe exit: %d within %s of stdin close\n", r.exitCode, r.exitWithin)
}

// spikeClient wraps the jrpc2 connection dial speaks directly (Design): an
// ad hoc handler map over channel.Line, not core.API/gencore.
type spikeClient struct {
	c     *jrpc2.Client
	event chan core.Event
}

func startClient(cmd *exec.Cmd) (*spikeClient, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	event := make(chan core.Event, 1)
	opts := &jrpc2.ClientOptions{
		OnNotify: func(req *jrpc2.Request) {
			var ev core.Event
			if req.Method() == eventRPC && req.UnmarshalParams(&ev) == nil {
				event <- ev
			}
		},
	}
	c := jrpc2.NewClient(channel.Line(stdout, stdin), opts)
	return &spikeClient{c: c, event: event}, nil
}

func (s *spikeClient) close() { _ = s.c.Close() }

func (s *spikeClient) ping(ctx context.Context) error {
	_, err := s.c.Call(ctx, pingRPC, nil)
	return err
}

func (s *spikeClient) warmPing(ctx context.Context) (time.Duration, error) {
	start := time.Now()
	for i := 0; i < warmPings; i++ {
		if err := s.ping(ctx); err != nil {
			return 0, fmt.Errorf("warm ping: %w", err)
		}
	}
	return time.Since(start) / warmPings, nil
}

func (s *spikeClient) awaitEvent(since time.Time) (time.Duration, json.RawMessage, error) {
	select {
	case ev := <-s.event:
		return time.Since(since), ev.Payload, nil
	case <-time.After(eventWait):
		return 0, nil, fmt.Errorf("event: not received within %s of connect", eventWait)
	}
}

func (s *spikeClient) fetchRows(ctx context.Context) (time.Duration, int, error) {
	start := time.Now()
	var rows []spikeRow
	if err := s.c.CallResult(ctx, rowsRPC, []int{spikeRowN}, &rows); err != nil {
		return 0, 0, fmt.Errorf("rows: %w", err)
	}
	dur := time.Since(start)
	b, err := json.Marshal(rows)
	if err != nil {
		return 0, 0, err
	}
	return dur, len(b), nil
}

// closeAndWait closes the client (which closes the channel's writer, i.e.
// the server's stdin) and waits for the child to exit.
func closeAndWait(client *spikeClient, cmd *exec.Cmd) (int, time.Duration, error) {
	start := time.Now()
	client.close()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		return exitCodeOf(err), time.Since(start), nil
	case <-time.After(exitWait):
		_ = cmd.Process.Kill()
		return 0, 0, fmt.Errorf("child did not exit within %s of stdin close", exitWait)
	}
}

func exitCodeOf(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 0
}
