// Requirements: FR-REPO-06, QA-PORT-02

package remote_test

import (
	"testing"

	"github.com/brogli/nebegit/internal/remote"
)

func TestBuildWSLPathRoundTripsThroughClassifyWSLPath(t *testing.T) {
	distro, linuxPath := "Ubuntu-22.04", "/home/user/project"

	uncPath := remote.BuildWSLPath(distro, linuxPath)
	gotDistro, gotPath, ok := remote.ClassifyWSLPath(uncPath)

	if !ok {
		t.Fatalf("ClassifyWSLPath(%q) ok = false", uncPath)
	}
	if gotDistro != distro || gotPath != linuxPath {
		t.Errorf("round trip = (%q, %q), want (%q, %q)", gotDistro, gotPath, distro, linuxPath)
	}
}

func TestClassifyWSLPath(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantDistro string
		wantPath   string
		wantOK     bool
	}{
		{
			name:       "wsl.localhost",
			path:       `\\wsl.localhost\Ubuntu-22.04\home\user\project`,
			wantDistro: "Ubuntu-22.04",
			wantPath:   "/home/user/project",
			wantOK:     true,
		},
		{
			name:       "legacy wsl$",
			path:       `\\wsl$\Debian\home\x`,
			wantDistro: "Debian",
			wantPath:   "/home/x",
			wantOK:     true,
		},
		{
			name:       "mixed-case prefix, distro case preserved",
			path:       `\\WSL.LOCALHOST\Ubuntu\a\b`,
			wantDistro: "Ubuntu",
			wantPath:   "/a/b",
			wantOK:     true,
		},
		{
			name:   "not a WSL path",
			path:   `C:\Users\x`,
			wantOK: false,
		},
		{
			name:   "distro root, no folder",
			path:   `\\wsl.localhost\Ubuntu-22.04`,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			distro, linuxPath, ok := remote.ClassifyWSLPath(tt.path)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if distro != tt.wantDistro || linuxPath != tt.wantPath {
				t.Errorf("got (%q, %q), want (%q, %q)", distro, linuxPath, tt.wantDistro, tt.wantPath)
			}
		})
	}
}
