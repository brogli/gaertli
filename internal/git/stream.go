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

	recordErr := scanRecords(stdout, splitNUL, onRecord)
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

// StreamErr runs call like Run, but instead of buffering stderr, calls
// onLine once per stderr record as it arrives, framed on the first \r or
// \n byte (git's own progress convention when run with --progress: \r
// overwrites the line currently being drawn, \n ends a completed one —
// confirmed empirically against git 2.43.0's own clone/fetch --progress
// output). Unlike Stream, stdout is buffered as Run's normally is — no job
// in scope needs both streamed at once.
//
// onLine's slice aliases an internal buffer and is only valid until onLine
// returns; copy anything kept past that. A non-nil error from onLine stops
// reading and kills the process, same contract as Stream's onRecord.
func (r Runner) StreamErr(ctx context.Context, call Call, onLine func([]byte) error) (Result, error) {
	cmd, effectiveCtx, cancel, command, err := r.prepare(ctx, call)
	if err != nil {
		result := Result{ExitCode: -1}
		r.log(call, command, result, 0, err)
		return result, err
	}
	defer cancel()

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	stderr, pipeErr := cmd.StderrPipe()
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

	var stderrText bytes.Buffer // kept whole for the error message a failure carries
	recordErr := scanRecords(stderr, splitCRLF, func(line []byte) error {
		stderrText.Write(line)
		stderrText.WriteByte('\n')
		return onLine(line)
	})
	if recordErr != nil {
		_ = cmd.Process.Kill()
	}
	runErr := cmd.Wait()
	duration := time.Since(start)

	result := Result{Stdout: stdout.Bytes(), Stderr: stderrText.Bytes(), ExitCode: cmd.ProcessState.ExitCode()}
	err = classify(effectiveCtx, cmd, runErr, result, command)
	if recordErr != nil {
		err = asCoreError(recordErr)
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
// dropping the delimiter — git's own progress-line convention, not
// bufio.ScanLines' \r\n-pair handling: git's intermediate updates end in a
// bare \r with no following \n.
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

// asCoreError wraps err as *core.Error{Code: Internal} unless it already is
// one.
func asCoreError(err error) error {
	var ce *core.Error
	if errors.As(err, &ce) {
		return err
	}
	return core.Errorf(core.Internal, "%v", err)
}
