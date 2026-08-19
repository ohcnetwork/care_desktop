{
  description = "CARE Desktop dev shell — Go/Wails/Node toolchain for app/";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      systems = [ "aarch64-darwin" "x86_64-darwin" "x86_64-linux" "aarch64-linux" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system} system);
    in
    {
      devShells = forAllSystems (pkgs: system:
        let
          isDarwin = pkgs.lib.hasSuffix "darwin" system;
          isLinux = pkgs.lib.hasSuffix "linux" system;
          # mkShellNoCC on Darwin: avoid nixpkgs' own cctools/SDK fighting with
          # the system Xcode CLT that `wails build` needs for cgo + WebKit.
          mkShell = if isDarwin then pkgs.mkShellNoCC else pkgs.mkShell;
        in
        {
          default = mkShell {
            packages = with pkgs; [
              go # 1.26.x — matches app/go.mod
              wails # CLI only; go.mod pins the wailsapp/wails/v2 *library* version separately,
                     # so a version mismatch warning from `wails build` is expected/harmless.
              nodejs_22 # builds the frontend (Vite); repo requires Node 20+ (nodejs_20 is EOL/removed)
              git
            ] ++ pkgs.lib.optionals isLinux [
              # Wails' Linux desktop build needs these (see docs/building.md);
              # nixpkgs doesn't have libwebkit2gtk-4.0, only the 4.1 API.
              pkg-config
              gtk3
              webkitgtk_4_1
            ];

            shellHook = ''
              echo "care_desktop dev shell: go $(go version | cut -d' ' -f3), wails $(wails version 2>/dev/null || echo '?'), node $(node --version)"
              ${pkgs.lib.optionalString isDarwin ''
                if ! xcode-select -p >/dev/null 2>&1; then
                  echo "warning: Xcode Command Line Tools not found — 'wails build' needs them (xcode-select --install)." >&2
                else
                  # The wails package pulls its own clang-wrapper onto PATH (it needs
                  # a C compiler for its own tray/icon tooling), which shadows Xcode's
                  # clang. cgo linking the Cocoa/WebKit frameworks needs the *system*
                  # clang + SDK, not nixpkgs' — so pin both explicitly.
                  export CC="$(xcrun --find clang)"
                  export SDKROOT="$(xcrun --show-sdk-path)"
                fi
              ''}
            '';
          };
        }
      );
    };
}
