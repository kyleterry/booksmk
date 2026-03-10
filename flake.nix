{
  description = "booksmk dev environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gotools
            golangci-lint

            sqlc
            templ

            postgresql

            air
            jq
          ];

          shellHook = ''
            export PGDATA="$PWD/.pgdata"
            export PGHOST="$PWD/.pgrun"
            export BOOKSMK_DATABASE_URL="postgres:///booksmk?host=$PGHOST"
          '';
        };
      }
    );
}
