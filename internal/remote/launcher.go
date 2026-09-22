package remote

import (
	"context"
	"path"
	"strings"

	"github.com/brogli/nebegit/internal/core"
)

// InstallLauncher makes `nebegit <path>` work inside d's own distro shell
// (ADR-3: "links ~/.local/bin/nebegit to the deployed core"): ensures the
// core binary is deployed (one consent ask, reused from ensureDeployed),
// records guiExePath's WSL-mount form next to the deployed core (so a
// future launch knows which Windows binary to hand off to, without
// depending on this distro's own $PATH containing a portable,
// no-installer .exe), then symlinks ~/.local/bin/nebegit to the deployed
// core.
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
