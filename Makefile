export CGO_ENABLED=0
export GOTOOLCHAIN=local

MODULE   = $(shell $(GO) list -m)
VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || \
			cat .version 2> /dev/null || echo v0)
PKGS     = $(or $(PKG),$(shell env GO111MODULE=on $(GO) list ./...))

GO      = go
NPM     = npm
TIMEOUT = 45
LSFILES = git ls-files -cmo --exclude-standard --
V = 0
Q = $(if $(filter 1,$V),,@)
M = $(shell if [ "$$(tput colors 2> /dev/null || echo 0)" -ge 8 ]; then printf "\033[34;1m▶\033[0m"; else printf "▶"; fi)

GENERATED_JS = \
	console/frontend/node_modules
GENERATED_GO = \
	common/pb/rawflow.pb.go \
	common/pb/rawflow_vtproto.pb.go \
	common/schema/definition_gen.go \
	orchestrator/clickhouse/data/asns.csv \
	orchestrator/clickhouse/data/protocols.csv \
	orchestrator/clickhouse/data/tcp.csv \
	orchestrator/clickhouse/data/udp.csv \
	console/filter/parser.go \
	outlet/core/asnprovider_enumer.go \
	outlet/core/netprovider_enumer.go \
	outlet/metadata/provider/snmp/authprotocol_enumer.go \
	outlet/metadata/provider/snmp/privprotocol_enumer.go \
	outlet/metadata/provider/gnmi/ifspeedpathunit_enumer.go \
	console/homepagetopwidget_enumer.go \
	common/kafka/saslmechanism_enumer.go
GENERATED_TEST_GO = \
	common/clickhousedb/mocks/mock_driver.go \
	conntrackfixer/mocks/mock_conntrackfixer.go
GENERATED = \
	$(GENERATED_GO) \
	$(GENERATED_JS) \
	console/data/frontend

.PHONY: all all_indep
all: fmt lint all_indep ; $(info $(M) building executable…) @ ## Build program binary
	$Q env GOOS=$(TARGETOS) GOARCH=$(TARGETARCH) \
         $(if $(filter amd64,$(TARGETARCH)),GOAMD64=$(TARGETVARIANT),\
         $(if $(filter arm64,$(TARGETARCH)),GOARM64=$(TARGETVARIANT:%=%.0),\
         $(if $(filter arm,$(TARGETARCH)),GOARM=$(TARGETVARIANT:v%=%)))) \
	   $(GO) build \
		-tags release \
		-ldflags '-X $(MODULE)/common/helpers.AkvoradoVersion=$(VERSION)' \
		-o bin/$(basename $(MODULE)) main.go
all_indep: $(GENERATED)

# Tools

ENUMER = go tool enumer
GOCOV = go tool gocov
GOCOVXML = go tool gocov-xml
GOIMPORTS = go tool goimports
GOTESTSUM = go tool gotestsum
MOCKGEN = go tool mockgen
PIGEON = go tool pigeon
REVIVE = go tool revive
STATICCHECK = go tool staticcheck
WWHRD = go tool wwhrd
BUF = go run github.com/bufbuild/buf/cmd/buf@v1.55.1

# Generated files

.DELETE_ON_ERROR:

common/pb/rawflow_vtproto.pb.go: common/pb/rawflow.pb.go
%.pb.go: buf.gen.yaml %.proto ; $(info $(M) compiling protocol buffers $@…)
	$Q $(BUF) generate --path $(@:.pb.go=.proto)

common/clickhousedb/mocks/mock_driver.go: go.mod ; $(info $(M) generate mocks for ClickHouse driver…)
	$Q $(MOCKGEN) -package mocks -build_constraint "!release" -destination $@ \
		github.com/ClickHouse/clickhouse-go/v2/lib/driver Conn,Row,Rows,ColumnType
	$Q touch $@
conntrackfixer/mocks/mock_conntrackfixer.go: go.mod ; $(info $(M) generate mocks for conntrack-fixer…)
	$Q if [ `$(GO) env GOOS` = "linux" ]; then \
	   $(MOCKGEN) -package mocks -build_constraint "!release" -destination $@ \
		akvorado/conntrackfixer ConntrackConn,DockerClient ; \
		touch $@ ; \
	fi

outlet/core/asnprovider_enumer.go: go.mod outlet/core/config.go ; $(info $(M) generate enums for ASNProvider…)
	$Q $(ENUMER) -type=ASNProvider -text -transform=kebab -trimprefix=ASNProvider outlet/core/config.go
outlet/core/netprovider_enumer.go: go.mod outlet/core/config.go ; $(info $(M) generate enums for NetProvider…)
	$Q $(ENUMER) -type=NetProvider -text -transform=kebab -trimprefix=NetProvider outlet/core/config.go
outlet/metadata/provider/snmp/authprotocol_enumer.go: go.mod outlet/metadata/provider/snmp/config.go ; $(info $(M) generate enums for AuthProtocol…)
	$Q $(ENUMER) -type=AuthProtocol -text -transform=kebab -trimprefix=AuthProtocol outlet/metadata/provider/snmp/config.go
outlet/metadata/provider/snmp/privprotocol_enumer.go: go.mod outlet/metadata/provider/snmp/config.go ; $(info $(M) generate enums for PrivProtocol…)
	$Q $(ENUMER) -type=PrivProtocol -text -transform=kebab -trimprefix=PrivProtocol outlet/metadata/provider/snmp/config.go
outlet/metadata/provider/gnmi/ifspeedpathunit_enumer.go: go.mod outlet/metadata/provider/gnmi/config.go ; $(info $(M) generate enums for IfSpeedPathUnit…)
	$Q $(ENUMER) -type=IfSpeedPathUnit -text -transform=kebab -trimprefix=Speed outlet/metadata/provider/gnmi/config.go
console/homepagetopwidget_enumer.go: go.mod console/config.go ; $(info $(M) generate enums for HomepageTopWidget…)
	$Q $(ENUMER) -type=HomepageTopWidget -text -json -transform=kebab -trimprefix=HomepageTopWidget console/config.go
common/kafka/saslmechanism_enumer.go: go.mod common/kafka/config.go ; $(info $(M) generate enums for SASLMechanism…)
	$Q $(ENUMER) -type=SASLMechanism -text -transform=kebab -trimprefix=SASL common/kafka/config.go

common/schema/definition_gen.go: common/schema/definition.go common/schema/definition_gen.sh ; $(info $(M) generate column definitions…)
	$Q ./common/schema/definition_gen.sh > $@
	$Q $(GOIMPORTS) -w $@

console/filter/parser.go: console/filter/parser.peg ; $(info $(M) generate PEG parser for filters…)
	$Q $(PIGEON) -optimize-basic-latin $< > $@

console/frontend/node_modules: console/frontend/package.json console/frontend/package-lock.json
console/frontend/node_modules: ; $(info $(M) fetching node modules…)
	$Q (cd console/frontend ; $(NPM) ci --loglevel=error --no-audit --no-fund) && touch $@
console/data/frontend: $(GENERATED_JS)
console/data/frontend: $(shell $(LSFILES) console/frontend 2> /dev/null)
console/data/frontend: ; $(info $(M) building console frontend…)
	$Q cd console/frontend && $(NPM) run --silent build

ASNS_URL = https://vincentbernat.github.io/asn2org/asns.csv
PROTOCOLS_URL = http://www.iana.org/assignments/protocol-numbers/protocol-numbers-1.csv
SERVICES_URL = https://www.iana.org/assignments/service-names-port-numbers/service-names-port-numbers.csv
define caturl
$(if $(filter http://% https://%, $(1)),curl --retry 3 --no-progress-meter --location --fail $(1),cat $(1))
endef

orchestrator/clickhouse/data/asns.csv: ; $(info $(M) generate ASN map…)
	$Q $(call caturl,$(ASNS_URL)) | sed 's|,[^,]*$$||' > $@
	$Q test -s $@
orchestrator/clickhouse/data/protocols.csv: ; $(info $(M) generate protocol map…)
	$Q $(call caturl,$(PROTOCOLS_URL)) \
		| sed -nE -e "1 s/.*/proto,name,description/p" -e "2,$$ s/^([0-9]+,[^ ,]+,[^\",]+),.*/\1/p" \
		> $@
	$Q test -s $@
orchestrator/clickhouse/data/udp.csv orchestrator/clickhouse/data/tcp.csv: orchestrator/clickhouse/data/%.csv: ; $(info $(M) generate $* port numbers…)
	$Q $(call caturl,$(SERVICES_URL)) \
		| sed -nE -e "1 s/.*/port,name/p" -e "2,$$ s/^([^,]+),([0-9]+),$*,.*/\2,\1/p" \
		| awk -F',' '!seen[$$1]++' \
		> $@
	$Q test -s $@

changelog.md: docs/99-changelog.md # To be used by GitHub actions only.
	$Q >  $@ < docs/99-changelog.md \
		sed -n '/^## '$${GITHUB_REF##*/v}' -/,/^## /{//!p}'
	$Q >> $@ echo "**Docker image**: \`docker pull ghcr.io/$${GITHUB_REPOSITORY}:$${GITHUB_REF##*/v}\`"
	$Q >> $@ echo "**Full changelog**: https://github.com/$${GITHUB_REPOSITORY}/compare/v$$(< docs/99-changelog.md sed -n '/^## '$${GITHUB_REF##*/v}' -/,/^## /{s/^## \([0-9.a-z-]*\) -.*/\1/p}' | tail -1)...v$${GITHUB_REF##*/v}"

# Update default.pgo with the locally running "docker compose" instance.
# Use: "make -j default.pgo".
default.pgo: default-inlet.pgo default-outlet.pgo default-console.pgo
	$Q go tool pprof -proto $^ > $@
	$Q rm $^
default-%.pgo:
	$Q container=akvorado-akvorado-$*-1 ; \
	   ip=$$(docker container inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $$container) ; \
	   [ -n $$ip ] ; \
	   curl -so $@ "http://$$ip:8080/debug/pprof/profile?seconds=30"

# Tests

.PHONY: check test tests test-race test-short test-bench test-coverage
.PHONY: test-go test-go-units test-go-staticcheck test-js test-coverage-go test-coverage-js
check test tests: test-go test-js ## Run tests
test-coverage: test-coverage-go test-coverage-js ## Run coverage tests

test-go-units test-go-checks test-bench test-race test-coverage-go: .fmt-go~ .lint-go~ $(GENERATED) $(GENERATED_TEST_GO)
test-go: test-go-units test-go-checks ## Run Go tests
test-go-checks: ; $(info $(M) running Go static checks…)
	$Q $(STATICCHECK) -f stylish -checks inherit,-SA1012 $(PKGS)
test-go-units: ; $(info $(M) running Go tests$(GOTEST_MORE)…)
	$Q mkdir -p test/go
	$Q env PATH=$(dir $(abspath $(shell command -v $(GO)))):$(PATH) $(GOTESTSUM) \
        --junitfile test/go/tests.xml -- \
		-timeout $(TIMEOUT)s \
		$(GOTEST_ARGS) $(PKGS)
test-race: CGO_ENABLED=1
test-race: GOTEST_ARGS=-race
test-race: GOTEST_MORE=, with race detector
test-race: test-go  ## Run Go tests with race detector
test-short: GOTEST_ARGS=-short
test-short: GOTEST_MORE=, only short tests
test-short: test-go  ## Run only short Go tests
test-bench: ; $(info $(M) running benchmarks…) @ ## Run Go benchmarks
	$Q $(GO) test \
		-fullpath -timeout $(TIMEOUT)s -run=__absolutelynothing__ -bench=. -benchmem \
		$(PKGS) # -memprofile test/go/memprofile.out -cpuprofile test/go/cpuprofile.out
test-coverage-go: ; $(info $(M) running Go coverage tests…) @ ## Run Go coverage tests
	$Q mkdir -p test/go
	$Q env PATH=$(dir $(abspath $(shell command -v $(GO)))):$(PATH) $(GOTESTSUM) -- \
	    -fullpath \
		-coverpkg=$(shell echo $(PKGS) | tr ' ' ',') \
		-covermode=atomic \
		-coverprofile=test/go/profile.out.tmp $(PKGS)
	$Q GENERATED=$$(awk -F: '(NR > 1) {print $$1}' test/go/profile.out.tmp \
			| sort | uniq | sed "s+^$(MODULE)/++" \
			| xargs grep -l "^//.*DO NOT EDIT\.$$" \
			| sed "s+\(.*\)+^$(MODULE)/\1:+" | paste -s -d '|' -) ; \
	   if [ -n "$$GENERATED" ]; then grep -Ev "$$GENERATED" test/go/profile.out.tmp > test/go/profile.out ; \
	   else cp test/go/profile.out.tmp test/go/profile.out ; \
	   fi
	$Q $(GO) tool cover -html=test/go/profile.out -o test/go/coverage.html
	$Q $(GOCOV) convert test/go/profile.out | $(GOCOVXML) > test/go/coverage.xml
	@printf "Code coverage: "; \
		c=$$(echo "scale=1;$$(sed -En 's/^<coverage line-rate="([0-9.]+)".*/\1/p' test/go/coverage.xml) * 100 / 1" | bc -q) ; \
	    echo $$c ; [ $$c != 0 ]

test-js: .fmt-js~ .lint-js~ $(GENERATED_JS)
test-js: ; $(info $(M) running JS tests…) @ ## Run JS tests
	$Q cd console/frontend && $(NPM) run --silent type-check && $(NPM) run --silent test
test-coverage-js: ; $(info $(M) running JS coverage tests…) @ ## Run JS coverage tests
	$Q cd console/frontend && $(NPM) run --silent type-check && $(NPM) run --silent test -- --coverage

.PHONY: lint
lint: .lint-go~ .lint-js~ ## Run linting
.lint-go~: $(shell $(LSFILES) '*.go' 2> /dev/null) ; $(info $(M) running golint…)
	$Q $(REVIVE) -formatter stylish -set_exit_status ./...
	$Q touch $@
.lint-js~: $(shell $(LSFILES) '*.js' '*.ts' '*.vue' '*.html' 2> /dev/null)
.lint-js~: $(GENERATED_JS) ; $(info $(M) running jslint…)
	$Q cd console/frontend && $(NPM) run --silent lint
	$Q touch $@

.PHONY: fmt
fmt: .fmt-go~ .fmt-js~ ## Format all source files
.fmt-go~: $(shell $(LSFILES) '*.go' 2> /dev/null) ; $(info $(M) formatting Go code…)
	$Q $(GOIMPORTS) -local $(MODULE) -w $? < /dev/null
	$Q touch $@
.fmt-js~: $(shell $(LSFILES) '*.js' '*.ts' '*.vue' '*.html' 2> /dev/null)
.fmt-js~: $(GENERATED_JS) ; $(info $(M) formatting JS code…)
	$Q cd console/frontend && $(NPM) run --silent format
	$Q touch $@

# Misc

.PHONY: licensecheck
licensecheck: console/frontend/node_modules ; $(info $(M) check dependency licenses…) @ ## Check licenses
	$Q ! git grep -L SPDX-License-Identifier: "*.go" "*.ts" "*.js" || \
		(>&2 echo "*** Missing license identifiers!"; false)
	$Q err=0 ; $(GO) mod vendor && $(WWHRD) --quiet check || err=$$? ; rm -rf vendor/ ; exit $$err
	$Q cd console/frontend ; $(NPM) exec --no -- license-compliance \
		--production \
		--allow "$$(sed -n '/^allowlist:/,/^[a-z]/p' ../../.wwhrd.yml | sed -n 's/^  - //p' | paste -sd ";")" \
		--report detailed

.PHONY: clean
clean: ; $(info $(M) cleaning…)	@ ## Cleanup everything
	@rm -rf test $(GENERATED) *~ bin/akvorado

.PHONY: help
help:
	@grep -hE '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-17s\033[0m %s\n", $$1, $$2}'

.PHONY: version
version:
	@echo $(VERSION)

.PHONY: docker docker-dev
DOCKER_BUILD_OPTIONS =
docker: ; $(info $(M) build Docker image…) @ ## Build Docker image
	$Q docker build -f docker/Dockerfile $(DOCKER_BUILD_OPTIONS) \
		--build-arg VERSION=$(VERSION) -t ghcr.io/akvorado/akvorado:main .
docker-dev: all ; $(info $(M) build development Docker image…) @ ## Build development Docker image
	$Q docker build -f docker/Dockerfile.dev $(DOCKER_BUILD_OPTIONS) \
		--build-arg VERSION=$(VERSION) -t ghcr.io/akvorado/akvorado:main .

# This requires "skopeo". I fetch it from nix.
.PHONY: docker-upgrade-versions
docker-upgrade-versions: ; $(info $(M) check for Docker image updates…) @ ## Check for Docker image updates
	$Q sed -En 's/^\s*image:\s+(.+):(.+)\s+#\s+(.+)$$/\1 \2 \3/p' docker/versions.yml \
		| while read -r image version regex; do \
			latest=$$(nix run nixpkgs\#skopeo -- list-tags docker://"$$image" \
				| sed -En 's/\s+"(.*)",?/\1/p' \
				| grep -xP "$$regex" \
				| sort -Vr | head -1); \
			[ "$$version" = "$$latest" ] || { \
				>&2 echo "$$image $$version→$$latest"; \
				sed -i "s,$$image:$$version,$$image:$$latest," docker/versions.yml; }; \
		done
