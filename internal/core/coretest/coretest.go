// Package coretest holds test helpers for core types.
package coretest

import (
	"errors"
	"testing"

	"github.com/brogli/nebegit/internal/core"
)

// AssertCode fails the test unless err is a *core.Error with Code want.
func AssertCode(tb testing.TB, err error, want core.Code) {
	tb.Helper()
	_ = AsError(tb, err, want)
}

// AsError is AssertCode that also returns the *core.Error.
func AsError(tb testing.TB, err error, want core.Code) *core.Error {
	tb.Helper()
	ce, ok := errors.AsType[*core.Error](err)
	if !ok || ce.Code != want {
		tb.Fatalf("err = %v, want *core.Error{Code: %s}", err, want)
	}
	return ce
}

// DrainEvents discards whatever events are already buffered, without
// waiting.
func DrainEvents(events chan core.Event) {
	for {
		select {
		case <-events:
		default:
			return
		}
	}
}
