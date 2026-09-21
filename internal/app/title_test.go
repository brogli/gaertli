package app

import "testing"

func TestFR_WT_09_WindowTitle(t *testing.T) {
	cases := []struct{ path, branch, want string }{
		{"", "", "nebegit"},
		{"/x/project", "", "project"},
		{"/x/project", "main", "project [main]"},
	}
	for _, c := range cases {
		if got := windowTitle(c.path, c.branch); got != c.want {
			t.Errorf("windowTitle(%q, %q) = %q, want %q", c.path, c.branch, got, c.want)
		}
	}
}
