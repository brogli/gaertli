package app_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/brogli/nebegit/internal/app"
	"github.com/brogli/nebegit/internal/core"
)

// stubCore answers Version and records Summary's id. The embedded
// interface is nil, so any other call panics.
type stubCore struct {
	core.API
	v    core.Version
	seen *core.WorktreeID // the id Summary was called with, if set
}

func (s stubCore) Version(context.Context) (core.Version, error) { return s.v, nil }

func (s stubCore) Summary(_ context.Context, wt core.WorktreeID) (core.Summary, error) {
	if s.seen != nil {
		*s.seen = wt
	}
	return core.Summary{}, nil
}

// stubRouter routes every worktree to owner, which knows it as ownerID.
type stubRouter struct {
	local   core.API
	owner   core.API
	ownerID core.WorktreeID
	other   int
}

func (r *stubRouter) Local() core.API { return r.local }

func (r *stubRouter) CoreOf(context.Context, core.WorktreeID) (core.API, core.WorktreeID, error) {
	r.other++
	return r.owner, r.ownerID, nil
}

func (r *stubRouter) CoreFor(core.RepoPath) (core.API, error) { r.other++; return nil, nil }

func TestADR_3_ForwarderRoutesWorktreeLessCallsToTheLocalCore(t *testing.T) {
	r := &stubRouter{local: stubCore{v: core.Version{App: "local"}}}
	f := app.NewForwarder(r)

	v, err := f.Version(t.Context())
	if err != nil || v.App != "local" {
		t.Errorf("Version = %+v, %v; want the local core's", v, err)
	}
	if r.other != 0 {
		t.Errorf("CoreOf/CoreFor called %d times", r.other)
	}
}

func TestTR_28_ForwarderCallsTheCoreWithTheIDTheRouterReturns(t *testing.T) {
	var seen core.WorktreeID
	f := app.NewForwarder(&stubRouter{owner: stubCore{seen: &seen}, ownerID: "/home/u/r"})

	if _, err := f.Summary(t.Context(), `\\wsl.localhost\A\home\u\r`); err != nil {
		t.Fatal(err)
	}

	if seen != "/home/u/r" {
		t.Errorf("the core got %q, want the router's id /home/u/r", seen)
	}
}

// Checked against the two interfaces, not a list of names: every Facade
// method is a WindowAPI method, and no API method outside WindowAPI is on
// Facade.
func TestTR_28_FacadeMethodSetIsExactlyWindowAPI(t *testing.T) {
	facade := reflect.TypeFor[*app.Facade]()
	windowAPI := reflect.TypeFor[core.WindowAPI]()
	api := reflect.TypeFor[core.API]()

	for method := range facade.Methods() {
		name := method.Name
		if _, ok := windowAPI.MethodByName(name); !ok {
			t.Errorf("Facade has %s, not a core.WindowAPI method", name)
		}
	}
	for method := range api.Methods() {
		name := method.Name
		if _, ok := windowAPI.MethodByName(name); ok {
			continue
		}
		if _, ok := facade.MethodByName(name); ok {
			t.Errorf("Facade has %s, a core.API method outside core.WindowAPI", name)
		}
	}
}
