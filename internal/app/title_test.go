// Requirements: FR-WT-09

package app

import "testing"

func TestWindowTitle(t *testing.T) {
	cases := []struct{ path, branch, want string }{
		{"", "", "Gärtli"},
		{"/x/project", "", "project"},
		{"/x/project", "main", "project [main]"},
	}
	for _, c := range cases {
		if got := windowTitle(c.path, c.branch); got != c.want {
			t.Errorf("windowTitle(%q, %q) = %q, want %q", c.path, c.branch, got, c.want)
		}
	}
}
