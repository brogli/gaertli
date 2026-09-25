package remote

import (
	"strings"
	"testing"
)

func TestFR_REPO_06_GuiMountPath(t *testing.T) {
	tests := []struct {
		name   string
		guiExe string
		distro string
		want   string
	}{
		{"a Windows drive path", `C:\Apps\nebegit.exe`, "Ubuntu", "/mnt/c/Apps/nebegit.exe"},
		{"an exe inside the same distro's file system", `\\wsl.localhost\Ubuntu\home\u\bin\nebegit.exe`, "Ubuntu", "/home/u/bin/nebegit.exe"},
		{"the \\\\wsl$ form of the same distro", `\\wsl$\Ubuntu\home\u\bin\nebegit.exe`, "Ubuntu", "/home/u/bin/nebegit.exe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := guiMountPath(tt.guiExe, tt.distro)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("guiMountPath(%q, %q) = %q, want %q", tt.guiExe, tt.distro, got, tt.want)
			}
		})
	}
}

// The launcher only runs an exe on a Windows drive or inside the distro
// it's linked into; an exe inside another distro is unsupported.
func TestFR_REPO_06_GuiMountPathRefusesAnExeInsideAnotherDistro(t *testing.T) {
	_, err := guiMountPath(`\\wsl.localhost\Ubuntu\home\u\bin\nebegit.exe`, "Debian")
	if err == nil {
		t.Fatal("want an error naming both distros")
	}
	for _, want := range []string{"Ubuntu", "Debian"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to name %q", err, want)
		}
	}
}

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
