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
        l = builtins // pkgs.lib;
        nodejs = pkgs.nodejs_20;
        go = pkgs.go_latest;
        frontend = pkgs.buildNpmPackage.override { inherit nodejs; } {
          name = "akvorado-frontend";
          src = ./console/frontend;
          npmDepsHash = l.readFile ./nix/npmDepsHash.txt;
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
          hash = l.readFile ./nix/ianaServiceNamesHash.txt;
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
          vendorHash = l.readFile ./nix/vendorHash.txt;
          proxyVendor = true; # generated code may contain additional dependencies
          nativeBuildInputs = [ pkgs.zip ];
          buildPhase = ''
            cp -r ${frontend}/node_modules console/frontend/node_modules
            cp -r ${frontend}/data console/data/frontend

            touch .fmt-js~ .fmt.go~ .lint-js~ .lint-go~
            find . -print0 | xargs -0 touch -d @0

            export XDG_CACHE_HOME=$TMPDIR
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
        apps = l.attrsets.mapAttrs
          (name: value:
            let
              script = pkgs.writeShellScriptBin name value;
            in
            {
              type = "app";
              program = "${script}/bin/${name}";
            })
          rec {
            # Update various hashes
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
            # Run nix build depending on TARGETPLATFORM value (for Docker).
            build = ''
              case $TARGETPLATFORM in
                linux/amd64/v*) target=packages.x86_64-linux.backend-amd64v''${TARGETPLATFORM##*/v} ;;
                linux/amd64*) target=packages.x86_64-linux.backend ;;
                linux/arm64/v*) target=packages.aarch64-linux.backend-arm64v''${TARGETPLATFORM##*/v}_0 ;;
                linux/arm64*) target=packages.aarch64-linux.backend ;;
                *)
                  >&2 echo "Unknown target platform $TARGETPLATFORM"
                  exit 1
                  ;;
              esac
              nix build --print-build-logs ".#$target"
            '';
          };

        packages = {
          inherit backend frontend ianaServiceNames;
          default = backend;
        } // (l.optionalAttrs (system == "x86_64-linux")
          (l.attrsets.listToAttrs (l.lists.map
            (v: {
              name = "backend-amd64${v}";
              value = backend.overrideAttrs (old: { env.GOAMD64 = v; });
            })
            # See https://go.dev/wiki/MinimumRequirements#amd64
            [ "v1" "v2" "v3" "v4" ])))
        // (l.optionalAttrs (system == "aarch64-linux")
          (l.attrsets.listToAttrs (l.lists.map
            (v: {
              name = "backend-arm64${l.strings.replaceStrings ["."] ["_"] v}";
              value = backend.overrideAttrs (old: { env.GOARM64 = v; });
            })
            # See https://go.dev/wiki/MinimumRequirements#arm64
            ((l.lists.map (m: "v8.${l.toString m}") (l.lists.range 0 9)) ++
              (l.lists.map (m: "v9.${l.toString m}") (l.lists.range 0 5))))));

        # Activate with "nix develop"
        devShells.default = pkgs.mkShell {
          name = "akvorado-dev";
          nativeBuildInputs = [
            go
            nodejs
            pkgs.git
            pkgs.curl
            pkgs.zip
          ];
        };
      });
}
