package remote

import (
	"context"
	"fmt"
	"path"
	"strings"
)

// commandPath is where the nebegit command goes inside a distro: a link to
// the deployed core, which hands a path to the Windows app. $HOME is
// expanded by the distro's own sh.
const commandPath = "$HOME/.local/bin/nebegit"

// setUpCommand makes `nebegit [path]` in d's distro open the Windows app at
// d.guiExe. Run on every fresh connect, so a new version's core and a moved
// app take over the command. A failure is logged and does not fail the
// connect.
func (d *DistroCore) setUpCommand(ctx context.Context) {
	if err := d.linkCommand(ctx); err != nil {
		d.log.Warn("nebegit command not set up", "distro", d.distro, "err", err)
	}
}

// linkCommand, in one wsl.exe start, links commandPath to the deployed
// core and records the app's WSL mount path next to that core, where the
// launcher reads it (path, not filepath: these are the distro's POSIX
// paths).
func (d *DistroCore) linkCommand(ctx context.Context) error {
	mountPath, err := guiMountPath(d.guiExe, d.distro)
	if err != nil {
		return err
	}
	target := deployTarget(d.version)
	script := fmt.Sprintf("mkdir -p %s && ln -sf %s %s && cat > %s",
		path.Dir(commandPath), target, commandPath, path.Join(path.Dir(target), "gui-path"))
	cmd, err := d.run(ctx, d.distro, "sh", "-c", script)
	if err != nil {
		return err
	}
	cmd.Stdin = strings.NewReader(mountPath)
	return cmd.Run()
}

// guiMountPath is guiExe's path as distro's shell sees it: a Windows drive
// path's DrvFs mount, or, for an exe inside distro's own file system, its
// own POSIX path there (WSL interop starts a Windows exe from a Linux
// path). An exe inside another distro is unsupported.
func guiMountPath(guiExe, distro string) (string, error) {
	if exeDistro, linuxPath, ok := ClassifyWSLPath(guiExe); ok {
		if exeDistro != distro {
			return "", fmt.Errorf("the app lies inside distro %s; the nebegit command in %s needs the app on a Windows drive or inside %s", exeDistro, distro, distro)
		}
		return linuxPath, nil
	}
	mountPath, ok := windowsPathToMount(guiExe)
	if !ok {
		return "", fmt.Errorf("not an absolute Windows path: %q", guiExe)
	}
	return mountPath, nil
}
