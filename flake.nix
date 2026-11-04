{
  description = "nebegit dev shell: everything except Go";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      forSystem =
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          isLinux = pkgs.stdenv.hostPlatform.isLinux;
          # Both tools are pinned as `tool` directives in go.mod; these shims
          # let the Taskfile's bare `wails3` and the documented `task` resolve.
          goTool = name: pkgs.writeShellScriptBin name ''exec go tool ${name} "$@"'';
          # WebKitGTK starts its web-process sandbox through bubblewrap at a
          # store path compiled in. Ubuntu's AppArmor grants user namespaces
          # only to programs with a profile: the host's /usr/bin/bwrap has one
          # (25.04 and later), a store path never does. The shim hands over to
          # the host's bwrap where the store one is denied. replaceDependency
          # rewrites WebKitGTK to the shim without a rebuild (same-length name).
          bwrapShim = pkgs.writeTextFile {
            name = pkgs.bubblewrap.name;
            destination = "/bin/bwrap";
            executable = true;
            text = ''
              #!${pkgs.runtimeShell}
              real=${pkgs.bubblewrap}/bin/bwrap
              if [ -x /usr/bin/bwrap ] && ! "$real" --ro-bind / / --unshare-user true 2>/dev/null; then
                exec /usr/bin/bwrap "$@"
              fi
              exec "$real" "$@"
            '';
          };
          # The dev output's closure holds the library; rewriting from it keeps
          # headers, pkg-config files and the library consistent.
          webkitgtkWithHostBwrap = pkgs.replaceDependency {
            drv = pkgs.webkitgtk_6_0.dev;
            oldDependency = pkgs.bubblewrap;
            newDependency = bwrapShim;
          };
        in
        pkgs.mkShell {
          packages =
            with pkgs;
            [
              nodejs_26
              pnpm_12
              golangci-lint
              treefmt
              nixfmt
              (goTool "task")
              (goTool "wails3")
            ]
            ++ lib.optionals isLinux [
              pkg-config
              gtk4
              webkitgtkWithHostBwrap
              adwaita-icon-theme # mouse cursor for the X11 backend under WSLg
              playwright-driver.browsers
              nsis # makensis, to cross-build the Windows installer
            ];

          shellHook = ''
            if ! command -v go >/dev/null; then
              echo "nebegit: Go is not provided by this shell. Install Go $(sed -n 's/^go //p' go.mod) or newer natively (see CONTRIBUTING.md)." >&2
            fi
            export PLAYWRIGHT_BROWSERS_PATH=${pkgs.playwright-driver.browsers}
            export PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=true
            if [ "$(stat -f -c %T "''${TMPDIR:-/tmp}" 2>/dev/null)" = tmpfs ]; then
              # Go builds under TMPDIR. On a RAM-backed /tmp with a per-user
              # quota (Ubuntu 25.04 and later) a cgo build fails with "disk
              # quota exceeded".
              export GOTMPDIR="''${XDG_CACHE_HOME:-$HOME/.cache}/go-tmp"
              mkdir -p "$GOTMPDIR"
            fi
          ''
          + pkgs.lib.optionalString isLinux ''
            # Go's build cache ignores pkg-config output, so after a nixpkgs
            # bump it would relink against the old library paths. The same
            # -L dirs in CGO_LDFLAGS are part of its cache key.
            export CGO_LDFLAGS="$(pkg-config --libs-only-L gtk4 webkitgtk-6.0)"
            # WebKit's processes load the GL driver at /run/opengl-driver, which
            # is missing on most hosts and on others (NixOS, system-manager)
            # comes from another nixpkgs. A driver built against a newer glibc
            # than this shell's fails to load. Without a driver WebKit's web
            # process aborts with EGL_BAD_PARAMETER and the window stays blank.
            # EGL, GBM and DRI use this shell's Mesa instead.
            export __EGL_VENDOR_LIBRARY_FILENAMES=${pkgs.mesa}/share/glvnd/egl_vendor.d/50_mesa.json
            export GBM_BACKENDS_PATH=${pkgs.mesa}/lib/gbm
            export LIBGL_DRIVERS_PATH=${pkgs.mesa}/lib/dri
            if grep -qi microsoft /proc/version 2>/dev/null || [ -e /proc/driver/nvidia/version ]; then
              # WSL and NVIDIA's proprietary driver render with Mesa's software
              # driver. NVIDIA's libraries must match the host's kernel module,
              # so this shell cannot bring them.
              export WEBKIT_DISABLE_DMABUF_RENDERER=1
              source ${pkgs.mesa.llvmpipeHook}/nix-support/setup-hook
            fi
            if grep -qi microsoft /proc/version 2>/dev/null; then
              # WSLg: on its Wayland path the page loads but never paints with
              # the software driver; on X11 (XWayland) it renders.
              export GDK_BACKEND=x11
              # On X11 GTK draws the pointer from a cursor theme; without one
              # the mouse is invisible over the window.
              export XCURSOR_PATH=${pkgs.adwaita-icon-theme}/share/icons
              export XCURSOR_THEME=Adwaita
              export XCURSOR_SIZE=24
              # GTK 4's file dialog goes through xdg-desktop-portal; the host's
              # portal backend cannot attach its dialog to an XWayland parent
              # ("Unhandled parent window type") and the app waits forever.
              # In-process dialogs instead.
              export GDK_DEBUG=no-portals
            fi
            # Ubuntu 24.04 permits no bwrap to an unprofiled program (its
            # profile for /usr/bin/bwrap came with 25.04).
            if ! ${bwrapShim}/bin/bwrap --ro-bind / / --unshare-user true 2>/dev/null; then
              echo "nebegit: this host denies bwrap a user namespace, so the app built here cannot start its web-process sandbox. See CONTRIBUTING.md, 'Ubuntu 24.04'." >&2
            fi
          '';
        };
    in
    {
      devShells.x86_64-linux.default = forSystem "x86_64-linux";
      devShells.aarch64-linux.default = forSystem "aarch64-linux";
      devShells.aarch64-darwin.default = forSystem "aarch64-darwin";
    };
}
