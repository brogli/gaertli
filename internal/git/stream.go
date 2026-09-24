package git

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"time"

	"github.com/brogli/nebegit/internal/core"
)

// Stream runs call like Run, but calls onRecord once per record as output
// arrives instead of buffering stdout. call.Args must build a
// --format that ends each record with a literal NUL; Stream only frames on
// 0x00.
//
// onRecord's slice aliases an internal buffer and is only valid until
// onRecord returns; copy anything kept past that. A non-nil error from
// onRecord stops reading, kills the process, and is what Stream returns
// (wrapped as *core.Error{Code: Internal} if it is not already one).
//
// Result.Stdout is left empty; everything else matches Run.
func (r Runner) Stream(ctx context.Context, call Call, onRecord func([]byte) error) (Result, error) {
	return r.stream(ctx, call, false, onRecord)
}

// StreamErr runs call like Run, but instead of buffering stderr, calls
// onLine once per stderr record as it arrives, framed on the first \r or
// \n byte (git's --progress convention: \r overwrites the line being drawn,
// \n ends a completed one). stdout is buffered as in Run.
//
// onLine's slice aliases an internal buffer and is only valid until onLine
// returns; copy anything kept past that. A non-nil error from onLine stops
// reading and kills the process, same contract as Stream's onRecord.
func (r Runner) StreamErr(ctx context.Context, call Call, onLine func([]byte) error) (Result, error) {
	return r.stream(ctx, call, true, onLine)
}

// stream is Stream's and StreamErr's shared body: it scans stdout, or
// stderr if fromStderr, as it arrives and buffers the other. Scanned stderr
// lines are kept whole too, for the error a failure carries.
//
// exec copies the scanned output into a pipe, so Wait's WaitDelay also
// covers it: a grandchild (a stalled ssh, a transport's delay) that still
// holds the output after git exited or was killed is cut off.
func (r Runner) stream(ctx context.Context, call Call, fromStderr bool, onRecord func([]byte) error) (Result, error) {
	ctx, cancel := withTimeout(ctx, call.Timeout)
	defer cancel()
	cmd, command, err := r.prepare(ctx, call)
	if err != nil {
		return r.notStarted(call, command, err)
	}

	var stdout, stderr bytes.Buffer
	records, output := io.Pipe()
	split, scan := splitNUL, onRecord
	if fromStderr {
		cmd.Stdout, cmd.Stderr = &stdout, output
		split = splitCRLF
		scan = func(line []byte) error {
			stderr.Write(line)
			stderr.WriteByte('\n')
			return onRecord(line)
		}
	} else {
		cmd.Stdout, cmd.Stderr = output, &stderr
	}
	if err := cmd.Start(); err != nil {
		return r.notStarted(call, command, core.Errorf(core.Internal, "%v", err))
	}

	scanned := make(chan error, 1)
	go func() {
		err := scanRecords(records, split, scan)
		if err != nil {
			cancel()                    // stops git
			records.CloseWithError(err) // and exec's copy into output
		}
		scanned <- err
	}()

	start := time.Now()
	runErr := cmd.Wait()
	duration := time.Since(start)
	_ = output.Close()
	recordErr := <-scanned

	result := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: cmd.ProcessState.ExitCode()}
	err = classify(ctx, cmd, runErr, result, command)
	if recordErr != nil {
		err = core.ErrorOf(recordErr)
	}
	r.log(call, command, result, duration, err)
	return result, err
}

// scanRecords reads r's records as framed by split and calls onRecord for
// each, stopping at onRecord's first error.
func scanRecords(r io.Reader, split bufio.SplitFunc, onRecord func([]byte) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	scanner.Split(split)
	for scanner.Scan() {
		if err := onRecord(scanner.Bytes()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// splitNUL is a bufio.SplitFunc that frames on 0x00, stripping the single
// leading '\n' git appends after every record's own terminator.
func splitNUL(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return i + 1, bytes.TrimPrefix(data[:i], []byte{'\n'}), nil
	}
	if !atEOF {
		return 0, nil, nil
	}
	if rest := bytes.TrimPrefix(data, []byte{'\n'}); len(rest) > 0 {
		return len(data), rest, nil
	}
	return 0, nil, nil
}

// splitCRLF is a bufio.SplitFunc that frames on the first \r or \n byte,
// dropping the delimiter. Unlike bufio.ScanLines it treats a bare \r as an
// end: git's intermediate progress updates end in one.
func splitCRLF(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if !atEOF {
		return 0, nil, nil
	}
	if len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}
