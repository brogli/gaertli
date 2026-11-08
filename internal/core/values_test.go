// Requirements: QA-MNT-07, FR-BR-01, FR-BR-02, FR-BR-09, FR-LOG-06

package core_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/brogli/gaertli/internal/core"
)

func TestSHAValidateAcceptsOnlyAFullLowercaseObjectName(t *testing.T) {
	for _, tc := range []struct {
		sha  core.SHA
		want bool
	}{
		{core.SHA(strings.Repeat("a1", 20)), true},
		{core.SHA(strings.Repeat("0f", 32)), true},
		{core.SHA(strings.Repeat("A1", 20)), false},
		{core.SHA(strings.Repeat("a", 39)), false},
		{core.SHA(strings.Repeat("g", 40)), false},
		{"main", false},
		{"", false},
	} {
		err := tc.sha.Validate()
		if (err == nil) != tc.want {
			t.Errorf("%q: Validate() = %v, want valid %v", tc.sha, err, tc.want)
		}
		var ce *core.Error
		if err != nil && (!errors.As(err, &ce) || ce.Code != core.Internal) {
			t.Errorf("%q: error = %v, want code %s", tc.sha, err, core.Internal)
		}
	}
}

func TestRefNameTellsItsNamespaceAndShortName(t *testing.T) {
	for _, tc := range []struct {
		ref                       core.RefName
		valid                     bool
		short                     string
		branch, remoteBranch, tag bool
	}{
		{"refs/heads/feature/x", true, "feature/x", true, false, false},
		{"refs/remotes/origin/main", true, "origin/main", false, true, false},
		{"refs/tags/v1.0", true, "v1.0", false, false, true},
		{"main", false, "", false, false, false},
		{"refs/stash", false, "", false, false, false},
		{"refs/heads/", false, "", true, false, false},
	} {
		if err := tc.ref.Validate(); (err == nil) != tc.valid {
			t.Errorf("%q: Validate() = %v, want valid %v", tc.ref, err, tc.valid)
		}
		if got := tc.ref.Short(); got != tc.short {
			t.Errorf("%q: Short() = %q, want %q", tc.ref, got, tc.short)
		}
		if got := tc.ref.IsBranch(); got != tc.branch {
			t.Errorf("%q: IsBranch() = %v", tc.ref, got)
		}
		if got := tc.ref.IsRemoteBranch(); got != tc.remoteBranch {
			t.Errorf("%q: IsRemoteBranch() = %v", tc.ref, got)
		}
		if got := tc.ref.IsTag(); got != tc.tag {
			t.Errorf("%q: IsTag() = %v", tc.ref, got)
		}
	}
}

func TestBranchRefNameIsTheLocalBranchsFullName(t *testing.T) {
	if got := core.BranchRefName("feature"); got != "refs/heads/feature" {
		t.Errorf("BranchRefName = %q", got)
	}
}
