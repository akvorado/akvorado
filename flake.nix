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
        go = pkgs.go_1_23;
        frontend = pkgs.buildNpmPackage.override { inherit nodejs; } {
          name = "akvorado-frontend";
          src = ./console/frontend;
          npmDepsHash = builtins.readFile ./nix/npmDepsHash.txt;
          installPhase = ''
            mkdir $out
            cp -r node_modules $out/node_modules
            cp -r ../data/frontend $out/data
          '';
        };
        ianaServiceNames = pkgs.fetchurl {
          url = "https://www.iana.org/assignments/service-names-port-numbers/service-names-port-numbers.csv";
          hash = builtins.readFile ./nix/ianaServiceNamesHash.txt;
        };
        backend = pkgs.buildGoModule.override { inherit go; } {
          doCheck = false;
          name = "akvorado";
          src = ./.;
          vendorHash = builtins.readFile ./nix/vendorHash.txt;
          buildPhase = ''
            cp -r ${frontend}/node_modules console/frontend/node_modules
            cp -r ${frontend}/data console/data/frontend

            touch .fmt-js~ .fmt.go~ .lint-js~ .lint-go~
            find . -print0 | xargs -0 touch -d @0

            make all \
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
                          | ${pkgs.gnused}/bin/sed -nE "s/\s+got:\s+(sha256-.*)/\1/p")
              [[ -z "$sha256" ]] || echo $sha256 > nix/vendorHash.txt
            '';
            update-npmDepsHash = ''
              sha256=$(2>&1 nix build --no-link .#frontend.npmDeps \
                          | ${pkgs.gnused}/bin/sed -nE "s/\s+got:\s+(sha256-.*)/\1/p")
              [[ -z "$sha256" ]] || echo $sha256 > nix/npmDepsHash.txt
            '';
            update-ianaServiceNamesHash = ''
              sha256=$(2>&1 nix build --no-link .#ianaServiceNames \
                          | ${pkgs.gnused}/bin/sed -nE "s/\s+got:\s+(sha256-.*)/\1/p")
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
