{
  description = "nebegit dev shell: everything except Go (TR-24, TR-25)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";
    # Only for golangci-lint: it must be built with a Go at least as new as the
    # native one, and stable lags a Go release behind.
    nixpkgs-unstable.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs, nixpkgs-unstable }:
    let
      forSystem = system:
        let
          pkgs = import nixpkgs { inherit system; };
          unstable = import nixpkgs-unstable { inherit system; };
          isLinux = pkgs.stdenv.isLinux;
          # Both tools are pinned as `tool` directives in go.mod; these shims
          # let the Taskfile's bare `wails3` and the documented `task` resolve.
          goTool = name: pkgs.writeShellScriptBin name ''exec go tool ${name} "$@"'';
        in
        pkgs.mkShell {
          packages = with pkgs; [
            nodejs_24
            pnpm_10
            unstable.golangci-lint
            (goTool "task")
            (goTool "wails3")
          ] ++ lib.optionals isLinux [
            pkg-config
            gtk4
            webkitgtk_6_0
          ];

          shellHook = ''
            if ! command -v go >/dev/null; then
              echo "nebegit: Go is not provided by this shell. Install Go $(sed -n 's/^go //p' go.mod) or newer natively (see CONTRIBUTING.md)." >&2
            fi
            if grep -qi microsoft /proc/version 2>/dev/null; then
              export WEBKIT_DISABLE_DMABUF_RENDERER=1
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
