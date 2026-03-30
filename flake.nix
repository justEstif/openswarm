{
  description = "openswarm — open, composable, file-backed multi-agent orchestration CLI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
  };

  outputs = inputs @ { flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];

      perSystem = { pkgs, ... }: {
        # Dev shell: everything needed to build, test, and lint
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            gotools          # goimports, godoc, etc.
            golangci-lint
            delve            # debugger
            goreleaser
            git
            gh
          ];

          shellHook = ''
            export GOPATH="$HOME/.cache/go"
            echo "openswarm dev shell ($(go version | cut -d' ' -f3))"
          '';
        };

        # Build the swarm binary
        packages.default = pkgs.buildGoModule {
          pname = "swarm";
          version = "0.1.0-dev";
          src = ./.;
          vendorHash = null; # update after go mod vendor
          subPackages = [ "cmd/swarm" ];

          meta = {
            description = "Open, composable, file-backed multi-agent orchestration CLI";
            homepage = "https://github.com/justEstif/openswarm";
            license = pkgs.lib.licenses.mit;
            mainProgram = "swarm";
          };
        };
      };
    };
}
