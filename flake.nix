{
  inputs = {
    nixpkgs.url = "nixpkgs";
    flake-utils.url = "github:numtide/flake-utils";
  };
  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages."${system}";
        deps = [
          pkgs.curl
          pkgs.git
          pkgs.go_1_19
          pkgs.nodejs-16_x
          pkgs.protobuf
        ];
      in
      {
        # This can be built with "nix build --option sandbox false".
        # Not a good example on how to package things for Nix!
        packages.default = pkgs.stdenv.mkDerivation {
          name = "akvorado";
          src = ./.;
          nativeBuildInputs = deps;
          configurePhase = ''
            export HOME=$TMPDIR
            export SSL_CERT_FILE=${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt
            export GOFLAGS=-trimpath
          '';
          installPhase = ''
            mkdir -p $out/bin
            cp bin/akvorado $out/bin/.
          '';
        };

        # Activate with "nix develop"
        devShells.default = pkgs.mkShell {
          name = "akvorado-dev";
          buildInputs = deps;
        };
      });
}
