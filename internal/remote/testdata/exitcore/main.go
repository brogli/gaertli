// Command exitcore is a test fixture: a normal core that exits without
// answering three calls. Jobs first sends 2000 events and writes 2000 stderr
// lines, so a test can check that everything a core writes before it exits
// is read. Refs first writes a stderr line longer than the logger reads.
// Summary first writes a line that is no JSON-RPC message, which
// ends the client's reading, and more output after it.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/services"
)

const burst = 2000

func main() {
	_ = remote.Serve(context.Background(), os.Stdin, os.Stdout, func(sink core.EventSink) core.API {
		return exiting{services.New(sink), sink}
	})
}

type exiting struct {
	*services.Core
	sink core.EventSink
}

func (e exiting) Jobs(context.Context, core.WorktreeID) ([]core.JobInfo, error) {
	for i := range burst {
		e.sink(core.Event{Name: fmt.Sprintf("event %d", i)})
		fmt.Fprintf(os.Stderr, "line %d\n", i)
	}
	os.Exit(0)
	return nil, nil
}

func (exiting) Refs(context.Context, core.WorktreeID) (core.Refs, error) {
	fmt.Fprintln(os.Stderr, strings.Repeat("x", 100*1024))
	fmt.Fprintln(os.Stderr, "more output")
	os.Exit(0)
	return core.Refs{}, nil
}

func (exiting) Summary(context.Context, core.WorktreeID) (core.Summary, error) {
	fmt.Println("not a message")
	time.Sleep(100 * time.Millisecond)
	fmt.Println("more output")
	os.Exit(0)
	return core.Summary{}, nil
}
