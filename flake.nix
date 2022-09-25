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
        curlMinimal = (pkgs.curlMinimal.override {
          http2Support = false;
          opensslSupport = false;
          zlibSupport = false;
          gssSupport = false;
        }).overrideAttrs
          (old: {
            configureFlags = old.configureFlags ++ [
              "--without-ssl"
              "--disable-dict"
              "--disable-file"
              "--disable-ftp"
              "--disable-gopher"
              "--disable-imap"
              "--disable-mqtt"
              "--disable-pop3"
              "--disable-rtsp"
              "--disable-smtp"
              "--disable-telnet"
              "--disable-tftp"
            ];
          });
      in
      {
        apps = {
          curl = {
            type = "app";
            program = "${curlMinimal}/bin/curl";
          };
        };
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
            ln -s ${curlMinimal}/bin/curl $out/bin/.
          '';
        };

        # Activate with "nix develop"
        devShells.default = pkgs.mkShell {
          name = "akvorado-dev";
          buildInputs = deps;
        };
      });
}
