// Package coretest holds test assertions on core types.
package coretest

import (
	"errors"
	"testing"

	"github.com/brogli/nebegit/internal/core"
)

// AssertCode fails the test unless err is a *core.Error with Code want.
func AssertCode(t testing.TB, err error, want core.Code) {
	t.Helper()
	_ = AsError(t, err, want)
}

// AsError is AssertCode that also returns the *core.Error.
func AsError(t testing.TB, err error, want core.Code) *core.Error {
	t.Helper()
	ce, ok := errors.AsType[*core.Error](err)
	if !ok || ce.Code != want {
		t.Fatalf("err = %v, want *core.Error{Code: %s}", err, want)
	}
	return ce
}
