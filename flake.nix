{
  inputs = {
    nixpkgs.url = "nixpkgs";
    flake-utils.url = "github:numtide/flake-utils";
    asn2org = {
      url = "github:vincentbernat/asn2org/gh-pages";
      flake = false;
    };
  };
  outputs = { self, nixpkgs, flake-utils, asn2org }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          # we use an overlay to set the desired versions of go and nodejs
          overlays = [
            (final: prev: {
              go = prev.go_1_20;
              nodejs = prev.nodejs-18_x;
            })
          ];
        };

        frontend = pkgs.buildNpmPackage {
          name = "akvorado-frontend";
          src = ./console/frontend;
          npmDepsHash = "sha256-xs2WHPrQFPtcjYEpB2Fb/gegP6Mf9ZD0VK/DcPg1zS8=";
          installPhase = ''
            mkdir $out
            cp -r node_modules $out/node_modules
            cp -r ../data/frontend $out/data
          '';
        };

        akvorado = pkgs.buildGoModule {
          doCheck = false;
          name = "akvorado";
          src = ./.;
          vendorHash = "sha256-0IO+mWdMTTPKgn1sisiRjT6uKXtxYOta8Uk9csi1604=";
          buildPhase = ''
            cp ${asn2org}/asns.csv orchestrator/clickhouse/data/asns.csv
            cp -r ${frontend}/node_modules console/frontend/node_modules
            cp -r ${frontend}/data console/data/frontend

            touch .fmt-js~ .fmt.go~ .lint-js~ .lint-go~
            find . -print0 | xargs -0 touch -d @0

            make all \
              MOCKGEN=${pkgs.mockgen}/bin/mockgen \
              GOIMPORTS=${pkgs.gotools}/bin/goimports \
              PIGEON=${pkgs.pigeon}/bin/pigeon \
              REVIVE=${pkgs.coreutils}/bin/true
          '';
          # We do not use a wrapper to set SSL_CERT_FILE because, either a
          # binary or a shell wrapper, it would pull the libc (~30M).
          installPhase = ''
            mkdir -p $out/bin $out/share/ca-certificates
            cp bin/akvorado $out/bin/.
            cp ${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt $out/share/ca-certificates/.
          '';
          # passthru is a special attribute that is not passed to the builder,
          # changing anything here does not invalidate the derivation output
          passthru = {
            # updates (in-place) vendorHash, which is needed to fetch go
            # dependencies, run with
            # nix run .#default.passthru.update-vendorHash
            update-vendorHash = pkgs.writeShellScriptBin "update-vendorHash" ''
              VENDOR_DIR=$(mktemp -d)
              ${pkgs.go}/bin/go mod vendor -v -o $VENDOR_DIR
              NEW_VENDOR_HASH=$(${pkgs.nix}/bin/nix hash path $VENDOR_DIR)
              ${pkgs.gnused}/bin/sed -i "s,${akvorado.vendorHash},$NEW_VENDOR_HASH," flake.nix
            '';
            # updates (in-place) npmDepsHash, which is needed to fetch nodejs
            # dependencies, run with
            # nix run .#default.passthru.update-npmDepsHash
            update-npmDepsHash = pkgs.writeShellScriptBin "update-npmDepsHash" ''
              NEW_DEPS_HASH=$(${pkgs.prefetch-npm-deps}/bin/prefetch-npm-deps ./console/frontend/package-lock.json)
              ${pkgs.gnused}/bin/sed -i "s,${frontend.npmDepsHash},$NEW_DEPS_HASH," flake.nix
            '';
          };
        };
      in
      {
        packages = {
          default = akvorado;
        };

        # Activate with "nix develop"
        devShells.default = pkgs.mkShell {
          name = "akvorado-dev";
          nativeBuildInputs = [
            pkgs.go
            pkgs.nodejs
            pkgs.git
            pkgs.curl
            pkgs.gomod2nix
          ];
        };
      });
}
