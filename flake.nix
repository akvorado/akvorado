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
        };
        nodejs = pkgs.nodejs-18_x;
        go = pkgs.go_1_20;
        frontend = pkgs.buildNpmPackage.override { inherit nodejs; } {
          name = "akvorado-frontend";
          src = ./console/frontend;
          npmDepsHash = "sha256-xs2WHPrQFPtcjYEpB2Fb/gegP6Mf9ZD0VK/DcPg1zS8=";
          installPhase = ''
            mkdir $out
            cp -r node_modules $out/node_modules
            cp -r ../data/frontend $out/data
          '';
        };
        backend = pkgs.buildGoModule.override { inherit go; } {
          doCheck = false;
          name = "akvorado";
          src = ./.;
          vendorHash = "sha256-cxL3WuvSKpsutVS3k5kduEDdAvk1ZM2XVU6YjcT+OTk=";
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
          installPhase= ''
            mkdir -p $out/bin $out/share/ca-certificates
            cp bin/akvorado $out/bin/.
            cp ${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt $out/share/ca-certificates/.
          '';
        };
      in
      rec {
        apps = {
          update = let
            script = pkgs.writeShellScriptBin "nix-update-akvorado" ''
              # go
              sha256=$(2>&1 nix build --no-link .#backend.go-modules \
                          | ${pkgs.gnused}/bin/sed -nE "s/\s+got:\s+(sha256-.*)/\1/p")
              [[ -z "$sha256" ]] || \
                 ${pkgs.gnused}/bin/sed -Ei "s,^(\s+[v]endorHash =).*,\1 \"''${sha256}\";," flake.nix

              # npm
              sha256=$(2>&1 nix build --no-link .#frontend.npmDeps \
                          | ${pkgs.gnused}/bin/sed -nE "s/\s+got:\s+(sha256-.*)/\1/p")
              [[ -z "$sha256" ]] || \
                 ${pkgs.gnused}/bin/sed -Ei "s,^(\s+[n]pmDepsHash =).*,\1 \"''${sha256}\";," flake.nix

              # asn2org
              nix flake lock --update-input asn2org
            '';
            in {
              type = "app";
              program = "${script}/bin/nix-update-akvorado";
            };
        };

        packages = {
          inherit backend frontend;
          default = backend;
        };

        # Activate with "nix develop"
        devShells.default = pkgs.mkShell {
          name = "akvorado-dev";
          nativeBuildInputs = [
            go
            nodejs
            pkgs.git
            pkgs.curl
            pkgs.gomod2nix
          ];
        };
      });
}
