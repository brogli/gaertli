package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/brogli/nebegit/internal/core"
)

// Stream runs call like Run, but calls onRecord once per record as output
// arrives instead of buffering stdout (ADR-5). call.Args must build a
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
	cmd, effectiveCtx, cancel, command, err := r.prepare(ctx, call)
	if err != nil {
		result := Result{ExitCode: -1}
		r.log(call, command, result, 0, err)
		return result, err
	}
	defer cancel()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, pipeErr := cmd.StdoutPipe()
	if pipeErr == nil {
		pipeErr = cmd.Start()
	}

	start := time.Now()
	if pipeErr != nil {
		result := Result{ExitCode: -1}
		err := core.Errorf(core.Internal, "%v", pipeErr)
		r.log(call, command, result, time.Since(start), err)
		return result, err
	}

	recordErr := scanRecords(stdout, onRecord)
	if recordErr != nil {
		_ = cmd.Process.Kill()
	}
	runErr := cmd.Wait()
	duration := time.Since(start)

	result := Result{Stderr: stderr.Bytes(), ExitCode: cmd.ProcessState.ExitCode()}
	err = classify(effectiveCtx, cmd, runErr, result, command)
	if recordErr != nil {
		err = asCoreError(recordErr)
	}
	r.log(call, command, result, duration, err)
	return result, err
}

// scanRecords reads r's 0x00-terminated records and calls onRecord for
// each, stopping at onRecord's first error.
func scanRecords(r io.Reader, onRecord func([]byte) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	scanner.Split(splitNUL)
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

// asCoreError wraps err as *core.Error{Code: Internal} unless it already is
// one.
func asCoreError(err error) error {
	var ce *core.Error
	if errors.As(err, &ce) {
		return err
	}
	return core.Errorf(core.Internal, "%v", err)
}
