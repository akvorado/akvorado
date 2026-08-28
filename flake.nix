{
  inputs = {
    nixpkgs.url = "nixpkgs";
    asn2org = {
      url = "github:vincentbernat/asn2org/gh-pages";
      flake = false;
    };
    iana-assignments = {
      url = "github:larseggert/iana-assignments";
      flake = false;
    };
  };
  outputs = { self, nixpkgs, asn2org, iana-assignments }:
    let
      systems = nixpkgs.lib.systems.flakeExposed;
      forAllSystems = nixpkgs.lib.genAttrs systems;
      perSystem = forAllSystems (system:
        let
          pkgs = import nixpkgs {
            inherit system;
          };
          l = builtins // pkgs.lib;
          nodejs = pkgs."nodejs_${l.versions.major (l.fileContents ./console/frontend/.nvmrc)}";
          pnpmVersion = l.removePrefix "pnpm@"
            (l.importJSON ./console/frontend/package.json).packageManager;
          pnpm = pkgs."pnpm_${l.versions.major pnpmVersion}";
          goVersion = l.pipe (l.readFile ./go.mod) [
            (l.splitString "\n")
            (l.findFirst (l.hasPrefix "go ") (throw "no go directive in go.mod"))
            (l.removePrefix "go ")
          ];
          go = pkgs."go_${l.replaceStrings [ "." ] [ "_" ] (l.versions.majorMinor goVersion)}";
          frontend = pkgs.stdenvNoCC.mkDerivation rec {
            name = "akvorado-frontend";
            src = ./console/frontend;
            nativeBuildInputs = [
              nodejs
              pnpm
              pkgs.pnpmConfigHook
            ];

            pnpmDeps = pkgs.fetchPnpmDeps {
              inherit src pnpm;
              pname = name;
              buildInputs = [ nodejs ];
              fetcherVersion = 4;
              hash = l.readFile ./nix/npmDepsHash.txt;
            };

            buildPhase = ''
              pnpm run build
            '';
            installPhase = ''
              mkdir $out
              cp -r node_modules $out/node_modules
              cp -r ../data/frontend $out/data
            '';
          };
          ianaServiceNames = pkgs.runCommand "service-names-port-numbers.csv" { } ''
            > $out echo name,port,protocol
            >> $out \
            ${pkgs.xmlstarlet}/bin/xmlstarlet sel -t -m "_:registry/_:record[_:name and _:number]" \
              -v _:name -o , \
              -v _:number -o , \
              -v _:protocol -o , -n \
              ${iana-assignments}/service-names-port-numbers/service-names-port-numbers.xml
          '';
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
        {
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
                oldSha256=$(cat nix/npmDepsHash.txt)
                echo sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA= > nix/npmDepsHash.txt
                sha256=$(2>&1 nix build --no-link .#frontend.pnpmDeps \
                            | ${pkgs.gnused}/bin/sed -nE "s/\s+got:\s+(sha256-.*)/\1/p")
                [[ -z "$sha256" ]] && echo $oldSha256 || echo $sha256 > nix/npmDepsHash.txt
              '';
              update = ''
                ${update-vendorHash}
                ${update-npmDepsHash}
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
              pnpm
              pkgs.git
              pkgs.curl
              pkgs.zip
              pkgs.clang
            ];
          };
        });
    in
    {
      apps = forAllSystems (system: perSystem.${system}.apps);
      packages = forAllSystems (system: perSystem.${system}.packages);
      devShells = forAllSystems (system: perSystem.${system}.devShells);
    };
}
