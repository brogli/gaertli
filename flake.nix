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
        in
        pkgs.mkShell {
          packages =
            with pkgs;
            [
              nodejs_24
              pnpm_11
              golangci-lint
              treefmt
              nixfmt
              (goTool "task")
              (goTool "wails3")
            ]
            ++ lib.optionals isLinux [
              pkg-config
              gtk4
              webkitgtk_6_0
              adwaita-icon-theme # mouse cursor for the X11 backend under WSLg
              playwright-driver.browsers
              nsis # makensis, to cross-build the Windows installer
            ];

          shellHook = ''
            if ! command -v go >/dev/null; then
              echo "nebegit: Go is not provided by this shell. Install Go $(sed -n 's/^go //p' go.mod) or newer natively (see CONTRIBUTING.md)." >&2
            fi
            if grep -qi microsoft /proc/version 2>/dev/null; then
              # WSLg: the Nix-built libEGL finds no GL driver on a non-NixOS
              # host (no /run/opengl-driver), so WebKit's web process aborts
              # with EGL_BAD_PARAMETER and the window stays white. Mesa's
              # llvmpipe hook points EGL/GLX at Mesa's own software driver.
              # X11 (XWayland): on WSLg's Wayland path the page loads but never
              # paints with the software driver; on X11 it renders.
              export WEBKIT_DISABLE_DMABUF_RENDERER=1
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
              source ${pkgs.mesa.llvmpipeHook}/nix-support/setup-hook
            fi
            export PLAYWRIGHT_BROWSERS_PATH=${pkgs.playwright-driver.browsers}
            export PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=true
          ''
          + pkgs.lib.optionalString isLinux ''
            # WebKitGTK runs its web-process sandbox through this bwrap (the
            # path is compiled in). Ubuntu's AppArmor denies user namespaces to
            # programs without a profile and ships none for it.
            if ! ${pkgs.bubblewrap}/bin/bwrap --ro-bind / / --unshare-user true 2>/dev/null; then
              echo "nebegit: this host denies the shell's bwrap a user namespace, so the app built here cannot start its web-process sandbox. See CONTRIBUTING.md, 'Native Ubuntu'." >&2
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
