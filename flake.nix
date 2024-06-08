{
    inputs = {
        nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
        flake-utils.url = "github:numtide/flake-utils";
    };

    outputs = { nixpkgs, flake-utils, ... } @ inputs: flake-utils.lib.eachDefaultSystem (system:
        let
            pkgs = import nixpkgs { inherit system; };
            inherit (pkgs) lib stdenv mkShell fetchFromGitLab buildGoModule;
        in {
            packages.default = buildGoModule rec {
                pname = "mkimg";
                version = "2.0.1";

                src = builtins.path {
                    name = pname;
                    path = ./.;
                };

                vendorHash = "sha256-AipbeksiApEbTrjpR3TiazOemI/QnG3Zk0IbBphr3o4=";

                meta = {
                    description = "mkimg is a tiny tool to simplify the process of creating partitioned disk images.";
                    homepage = https://git.thenest.dev/wux/mkimg;
                    maintainers = with lib.maintainers; [ wux ];
                };
            };
            devShells.default = mkShell {
                shellHook = "export DEVSHELL_PS1_PREFIX=' mkimg'";
                nativeBuildInputs = [ pkgs.go ];
            };
        }
    );
}