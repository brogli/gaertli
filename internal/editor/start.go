package editor

import (
	"os/exec"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/proc"
)

// start launches argv[0] with argv[1:], detached (TR-12) — the caller never
// waits for the editor to exit.
func start(argv []string) error {
	bin, err := exec.LookPath(argv[0])
	if err != nil {
		return core.Errorf(core.EditorNotFound, "%q is not on PATH", argv[0])
	}
	cmd := exec.Command(bin, argv[1:]...)
	proc.Detach(cmd)
	if err := cmd.Start(); err != nil {
		return core.Errorf(core.EditorNotFound, "could not start %q: %v", bin, err)
	}
	return cmd.Process.Release()
}
