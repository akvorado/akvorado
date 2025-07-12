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
        nodejs = pkgs.nodejs_20;
        go = pkgs.go_1_24;
        frontend = pkgs.buildNpmPackage.override { inherit nodejs; } {
          name = "akvorado-frontend";
          src = ./console/frontend;
          npmDepsHash = builtins.readFile ./nix/npmDepsHash.txt;
          # Filter out optional dependencies
          prePatch = ''
            ${pkgs.jq}/bin/jq 'del(.packages[] | select(.optional == true and .dev == null))' \
              < package-lock.json > package-lock.json.tmp
            mv package-lock.json.tmp package-lock.json
          '';
          installPhase = ''
            mkdir $out
            cp -r node_modules $out/node_modules
            cp -r ../data/frontend $out/data
          '';
        };
        ianaServiceNames = pkgs.fetchurl {
          url = "https://www.iana.org/assignments/service-names-port-numbers/service-names-port-numbers.csv";
          hash = builtins.readFile ./nix/ianaServiceNamesHash.txt;
          # There are many bogus changes in this file. To avoid updating the
          # hash too often, filter the lines with a service name and a port.
          downloadToTemp = true;
          postFetch = ''
            < $downloadedFile > $out \
            awk -F, '(NR == 1) {print} ($0 !~ "^ " && $1 != "" && $2 != "" && ($3 == "tcp" || $3 == "udp")) {print}'
          '';
        };
        backend = pkgs.buildGoModule.override { inherit go; } {
          doCheck = false;
          name = "akvorado";
          src = ./.;
          vendorHash = builtins.readFile ./nix/vendorHash.txt;
          proxyVendor = true;   # generated code may contain additional dependencies
          buildPhase = ''
            cp -r ${frontend}/node_modules console/frontend/node_modules
            cp -r ${frontend}/data console/data/frontend

            touch .fmt-js~ .fmt.go~ .lint-js~ .lint-go~
            find . -print0 | xargs -0 touch -d @0

            make all \
              BUF=${pkgs.buf}/bin/buf \
              ASNS_URL=${asn2org}/asns.csv \
              SERVICES_URL=${ianaServiceNames}
          '';
          installPhase = ''
            mkdir -p $out/bin
            cp bin/akvorado $out/bin/.
          '';
        };
      in
      rec {
        apps = pkgs.lib.attrsets.mapAttrs
          (name: value:
            let
              script = pkgs.writeShellScriptBin name value;
            in
            {
              type = "app";
              program = "${script}/bin/${name}";
            })
          rec {
            update-vendorHash = ''
              sha256=$(2>&1 nix build --no-link .#backend.goModules \
                          | sed -nE "s/\s+got:\s+(sha256-.*)/\1/p")
              [[ -z "$sha256" ]] || echo $sha256 > nix/vendorHash.txt
            '';
            update-npmDepsHash = ''
              sha256=$(2>&1 nix build --no-link .#frontend.npmDeps \
                          | sed -nE "s/\s+got:\s+(sha256-.*)/\1/p")
              [[ -z "$sha256" ]] || echo $sha256 > nix/npmDepsHash.txt
            '';
            update-ianaServiceNamesHash = ''
              sha256=$(2>&1 nix build --no-link .#ianaServiceNames \
                          | sed -nE "s/\s+got:\s+(sha256-.*)/\1/p")
              [[ -z "$sha256" ]] || echo $sha256 > nix/ianaServiceNamesHash.txt
            '';
            update = ''
              ${update-vendorHash}
              ${update-npmDepsHash}
              ${update-ianaServiceNamesHash}
            '';
          };

        packages = {
          inherit backend frontend ianaServiceNames;
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
          ];
        };
      });
}
