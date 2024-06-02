export CGO_ENABLED=0
export GOEXPERIMENT=loopvar
export GOTOOLCHAIN=local

MODULE   = $(shell $(GO) list -m)
DATE    ?= $(shell date +%FT%T%z)
VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || \
			cat .version 2> /dev/null || echo v0)
PKGS     = $(or $(PKG),$(shell env GO111MODULE=on $(GO) list ./...))
BIN      = bin

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
	common/schema/definition_gen.go \
	orchestrator/clickhouse/data/asns.csv \
	console/filter/parser.go
GENERATED_TEST_GO = \
	common/clickhousedb/mocks/mock_driver.go \
	conntrackfixer/mocks/mock_conntrackfixer.go
GENERATED = \
	$(GENERATED_GO) \
	$(GENERATED_JS) \
	console/data/frontend

.PHONY: all
all: fmt lint $(GENERATED) | $(BIN) ; $(info $(M) building executable…) @ ## Build program binary
	$Q $(GO) build \
		-tags release \
		-ldflags '-X $(MODULE)/cmd.Version=$(VERSION)' \
		-o $(BIN)/$(basename $(MODULE)) main.go

.PHONY: all_js
all_js: .fmt-js~ .lint-js~ $(GENERATED_JS) console/data/frontend

# Tools

$(BIN):
	@mkdir -p $@
$(BIN)/%: | $(BIN) ; $(info $(M) building $(PACKAGE)…)
	$Q env GOBIN=$(abspath $(BIN)) $(GO) install $(PACKAGE)

GOIMPORTS = $(BIN)/goimports
$(BIN)/goimports: PACKAGE=golang.org/x/tools/cmd/goimports@latest

REVIVE = $(BIN)/revive
$(BIN)/revive: PACKAGE=github.com/mgechev/revive@latest

GOCOV = $(BIN)/gocov
$(BIN)/gocov: PACKAGE=github.com/axw/gocov/gocov@v1.1.0

GOCOVXML = $(BIN)/gocov-xml
$(BIN)/gocov-xml: PACKAGE=github.com/AlekSi/gocov-xml@latest

GOTESTSUM = $(BIN)/gotestsum
$(BIN)/gotestsum: PACKAGE=gotest.tools/gotestsum@latest

MOCKGEN = $(BIN)/mockgen
$(BIN)/mockgen: PACKAGE=go.uber.org/mock/mockgen@v0.4.0

PIGEON = $(BIN)/pigeon
$(BIN)/pigeon: PACKAGE=github.com/mna/pigeon@v1.1.0

WWHRD = $(BIN)/wwhrd
$(BIN)/wwhrd: PACKAGE=github.com/frapposelli/wwhrd@latest

# Generated files

.DELETE_ON_ERROR:

common/clickhousedb/mocks/mock_driver.go: go.mod | $(MOCKGEN) ; $(info $(M) generate mocks for ClickHouse driver…)
	$Q echo '//go:build !release' > $@
	$Q $(MOCKGEN) -package mocks \
		github.com/ClickHouse/clickhouse-go/v2/lib/driver Conn,Row,Rows,ColumnType >> $@
conntrackfixer/mocks/mock_conntrackfixer.go: go.mod | $(MOCKGEN) ; $(info $(M) generate mocks for conntrack-fixer…)
	$Q if [ `$(GO) env GOOS` = "linux" ]; then \
	   echo '//go:build !release' > $@ ; \
	   $(MOCKGEN) -package mocks akvorado/conntrackfixer ConntrackConn,DockerClient >> $@ ; \
	fi

common/schema/definition_gen.go: common/schema/definition.go common/schema/definition_gen.sh ; $(info $(M) generate column definitions…)
	$Q ./common/schema/definition_gen.sh > $@
	$Q $(GOIMPORTS) -w $@

console/filter/parser.go: console/filter/parser.peg | $(PIGEON) ; $(info $(M) generate PEG parser for filters…)
	$Q $(PIGEON) -optimize-basic-latin $< > $@

console/frontend/node_modules: console/frontend/package.json console/frontend/package-lock.json
console/frontend/node_modules: ; $(info $(M) fetching node modules…)
	$Q (cd console/frontend ; $(NPM) ci --loglevel=error --no-audit --no-fund) && touch $@
console/data/frontend: $(GENERATED_JS)
console/data/frontend: $(shell $(LSFILES) console/frontend 2> /dev/null)
console/data/frontend: ; $(info $(M) building console frontend…)
	$Q cd console/frontend && $(NPM) run --silent build

orchestrator/clickhouse/data/asns.csv: ; $(info $(M) generate ASN map…)
	$Q curl -sL https://vincentbernat.github.io/asn2org/asns.csv | sed 's|,[^,]*$$||' > $@
	$Q test -s $@
orchestrator/clickhouse/data/protocols.csv: # We keep this one in Git
	$Q curl -sL http://www.iana.org/assignments/protocol-numbers/protocol-numbers-1.csv \
		| sed -nE -e "1 s/.*/proto,name,description/p" -e "2,$ s/^([0-9]+,[^ ,]+,[^\",]+),.*/\1/p" \
		> $@
	$Q test -s $@

changelog.md: docs/99-changelog.md # To be used by GitHub actions only.
	$Q >  $@ < docs/99-changelog.md \
		sed -n '/^## '$${GITHUB_REF##*/v}' -/,/^## /{//!p}'
	$Q >> $@ echo "**Docker image**: \`docker pull ghcr.io/$${GITHUB_REPOSITORY}:$${GITHUB_REF##*/v}\`"
	$Q >> $@ echo "**Full changelog**: https://github.com/$${GITHUB_REPOSITORY}/compare/v$$(< docs/99-changelog.md sed -n '/^## '$${GITHUB_REF##*/v}' -/,/^## /{s/^## \([0-9.]*\) -.*/\1/p}' | tail -1)...v$${GITHUB_REF##*/v}"

# Tests

.PHONY: check test tests test-race test-short test-bench test-coverage
.PHONY: test-go test-js test-coverage-go test-coverage-js
check test tests: test-go test-js ## Run tests
test-coverage: test-coverage-go test-coverage-js ## Run coverage tests

test-go test-bench test-race test-coverage-go: .fmt-go~ .lint-go~ $(GENERATED) $(GENERATED_TEST_GO)
test-go: | $(GOTESTSUM) ; $(info $(M) running Go tests$(GOTEST_MORE)…) @ ## Run Go tests
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
test-coverage-go: | $(GOTESTSUM) $(GOCOV) $(GOCOVXML) ; $(info $(M) running Go coverage tests…) @ ## Run Go coverage tests
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
		echo "scale=1;$$(sed -En 's/^<coverage line-rate="([0-9.]+)".*/\1/p' test/go/coverage.xml) * 100 / 1" | bc -q

test-js: .fmt-js~ .lint-js~ $(GENERATED_JS)
test-js: ; $(info $(M) running JS tests…) @ ## Run JS tests
	$Q cd console/frontend && $(NPM) run --silent type-check && $(NPM) run --silent test
test-coverage-js: ; $(info $(M) running JS coverage tests…) @ ## Run JS coverage tests
	$Q cd console/frontend && $(NPM) run --silent type-check && $(NPM) run --silent test -- --coverage

.PHONY: lint
lint: .lint-go~ .lint-js~ ## Run linting
.lint-go~: $(shell $(LSFILES) '*.go' 2> /dev/null) | $(REVIVE) ; $(info $(M) running golint…)
	$Q $(REVIVE) -formatter friendly -set_exit_status ./...
	$Q touch $@
.lint-js~: $(shell $(LSFILES) '*.js' '*.ts' '*.vue' '*.html' 2> /dev/null)
.lint-js~: $(GENERATED_JS) ; $(info $(M) running jslint…)
	$Q cd console/frontend && $(NPM) run --silent lint
	$Q touch $@

.PHONY: fmt
fmt: .fmt-go~ .fmt-js~ ## Format all source files
.fmt-go~: $(shell $(LSFILES) '*.go' 2> /dev/null) | $(GOIMPORTS) ; $(info $(M) formatting Go code…)
	$Q $(GOIMPORTS) -local $(MODULE) -w $? < /dev/null
	$Q touch $@
.fmt-js~: $(shell $(LSFILES) '*.js' '*.ts' '*.vue' '*.html' 2> /dev/null)
.fmt-js~: $(GENERATED_JS) ; $(info $(M) formatting JS code…)
	$Q cd console/frontend && $(NPM) run --silent format
	$Q touch $@

# Misc

.PHONY: licensecheck
licensecheck: console/frontend/node_modules | $(WWHRD) ; $(info $(M) check dependency licenses…) @ ## Check licenses
	$Q ! git grep -L SPDX-License-Identifier: "*.go" "*.ts" "*.js" || \
		(>&2 echo "*** Missing license identifiers!"; false)
	$Q err=0 ; $(GO) mod vendor && $(WWHRD) --quiet check || err=$$? ; rm -rf vendor/ ; exit $$err
	$Q cd console/frontend ; $(NPM) exec --no -- license-compliance \
		--production \
		--allow "$$(sed -n 's/^  - //p' ../../.wwhrd.yml | paste -sd ";")" \
		--report detailed

.PHONY: clean mrproper
clean: ; $(info $(M) cleaning…)	@ ## Cleanup almost everything
	@rm -rf test $(GENERATED) inlet/flow/decoder/flow-*.pb.go *~
mrproper: clean
	@rm -rf bin

.PHONY: help
help:
	@grep -hE '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-17s\033[0m %s\n", $$1, $$2}'

.PHONY: version
version:
	@echo $(VERSION)

.PHONY: docker
docker: ; $(info $(M) build Docker image…) @ ## Build Docker image
	$Q docker build -f docker/Dockerfile -t ghcr.io/akvorado/akvorado:main .
