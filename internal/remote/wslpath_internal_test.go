package remote

import "testing"

func TestFR_REPO_06_WindowsPathToMount(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"C drive", `C:\Users\user\nebegit.exe`, "/mnt/c/Users/user/nebegit.exe", true},
		{"D drive", `D:\x\y.exe`, "/mnt/d/x/y.exe", true},
		{"UNC network share", `\\server\share\x.exe`, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := windowsPathToMount(tt.in)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("windowsPathToMount(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
