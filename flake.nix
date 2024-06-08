{
    inputs = {
        nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
        flake-utils.url = "github:numtide/flake-utils";
    };

    outputs = { nixpkgs, flake-utils, ... } @ inputs: flake-utils.lib.eachDefaultSystem (system:
        let
            pkgs = import nixpkgs { inherit system; };
            inherit (pkgs) lib stdenv fetchFromGitLab;
        in {
            packages.default = stdenv.mkDerivation {
                pname = "mkimg";
                version = "";

                src = fetchFromGitLab {
                    domain = "git.thenest.dev";
                    owner = "wux";
                    repo = pname;
                    rev = version;
                    hash = lib.fakeHash;
                };

                vendorHash = lib.fakeHash;

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