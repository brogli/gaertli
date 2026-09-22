package remote

import (
	"context"
	"path"
	"strings"

	"github.com/brogli/nebegit/internal/core"
)

// InstallLauncher makes `nebegit <path>` work inside d's distro shell: it
// deploys the core binary if needed, records guiExePath's
// WSL-mount form next to it so a later launch knows which Windows binary to
// hand off to, then symlinks ~/.local/bin/nebegit to the deployed core.
func (d *DistroCore) InstallLauncher(ctx context.Context, guiExePath string) error {
	if err := ensureDeployed(ctx, d.run, d.consent, d.distro, d.version); err != nil {
		return err
	}
	mountPath, ok := windowsPathToMount(guiExePath)
	if !ok {
		return core.Errorf(core.Internal, "not an absolute Windows path: %s", guiExePath)
	}

	target := deployTarget(d.version)
	guiPathFile := path.Join(path.Dir(target), "gui-path") // path, not filepath: this is the distro's own POSIX path

	write, err := d.run(ctx, d.distro, "sh", "-c", "cat > "+guiPathFile)
	if err != nil {
		return err
	}
	write.Stdin = strings.NewReader(mountPath)
	if err := write.Run(); err != nil {
		return err
	}

	link, err := d.run(ctx, d.distro, "sh", "-c", "mkdir -p ~/.local/bin && ln -sf "+target+" ~/.local/bin/nebegit")
	if err != nil {
		return err
	}
	return link.Run()
}
