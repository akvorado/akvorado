{
  inputs = {
    nixpkgs.url = "nixpkgs";
    flake-utils.url = "github:numtide/flake-utils";
  };
  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages."${system}";
      in
      {
        devShells.default = pkgs.mkShell {
          name = "akvorado-dev";
          buildInputs = [
            pkgs.curl
            pkgs.git
            pkgs.go
            pkgs.nodejs
            pkgs.protobuf
          ];
        };
      });
}
