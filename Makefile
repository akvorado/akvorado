MODULE   = $(shell $(GO) list -m)
DATE    ?= $(shell date +%FT%T%z)
VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || \
			cat .version 2> /dev/null || echo v0)
PKGS     = $(or $(PKG),$(shell env GO111MODULE=on $(GO) list ./...))
BIN      = bin

GO      = go
TIMEOUT = 15
LSFILES = git ls-files -cmo --exclude-standard --
V = 0
Q = $(if $(filter 1,$V),,@)
M = $(shell if [ "$$(tput colors 2> /dev/null || echo 0)" -ge 8 ]; then printf "\033[34;1m▶\033[0m"; else printf "▶"; fi)

FLOW_VERSION := $(shell sed -n 's/^const CurrentSchemaVersion = //p' inlet/flow/schemas.go)
GENERATED = \
	inlet/flow/decoder/flow-$(FLOW_VERSION).pb.go \
	common/clickhousedb/mocks/mock_driver.go \
	conntrackfixer/mocks/mock_conntrackfixer.go \
	orchestrator/clickhouse/data/asns.csv \
	console/filter/parser.go \
	console/data/frontend \
	console/frontend/node_modules \
	console/frontend/data/fields.json

.PHONY: all
all: fmt lint $(GENERATED) | $(BIN) ; $(info $(M) building executable…) @ ## Build program binary
	$Q $(GO) build \
		-tags release \
		-ldflags '-X $(MODULE)/cmd.Version=$(VERSION) -X $(MODULE)/cmd.BuildDate=$(DATE)' \
		-o $(BIN)/$(basename $(MODULE)) main.go

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

GCOV2LCOV = $(BIN)/gcov2lcov
$(BIN)/gcov2lcov: PACKAGE=github.com/jandelgado/gcov2lcov@latest

GOTESTSUM = $(BIN)/gotestsum
$(BIN)/gotestsum: PACKAGE=gotest.tools/gotestsum@latest

MOCKGEN = $(BIN)/mockgen
$(BIN)/mockgen: PACKAGE=github.com/golang/mock/mockgen@v1.6.0

PROTOC = protoc
PROTOC_GEN_GO = $(BIN)/protoc-gen-go
$(BIN)/protoc-gen-go: PACKAGE=google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.0

PIGEON = $(BIN)/pigeon
$(BIN)/pigeon: PACKAGE=github.com/mna/pigeon@v1.1.0

WWHRD = $(BIN)/wwhrd
$(BIN)/wwhrd: PACKAGE=github.com/frapposelli/wwhrd@latest

# Generated files

.DELETE_ON_ERROR:

inlet/flow/decoder/flow-%.pb.go: inlet/flow/data/schemas/flow-%.proto | $(PROTOC_GEN_GO) ; $(info $(M) compiling protocol buffers definition…)
	$Q $(PROTOC) -I=. --plugin=$(PROTOC_GEN_GO) --go_out=. --go_opt=module=$(MODULE) $<
	$Q sed -i.bkp s/FlowMessagev./FlowMessage/g $@ && rm $@.bkp
	$Q : $$((i=$*)); while [ $$i -ne 0 ]; do : $$((i = i - 1)); rm -f inlet/flow/decoder/flow-$$i.pb.go; done

common/clickhousedb/mocks/mock_driver.go: $(MOCKGEN) ; $(info $(M) generate mocks for ClickHouse driver…)
	$Q echo '//go:build !release' > $@
	$Q $(MOCKGEN) -package mocks \
		github.com/ClickHouse/clickhouse-go/v2/lib/driver Conn,Row,Rows,ColumnType >> $@
conntrackfixer/mocks/mock_conntrackfixer.go: $(MOCKGEN) ; $(info $(M) generate mocks for conntrack-fixer…)
	$Q if [ `$(GO) env GOOS` = "linux" ]; then \
	   echo '//go:build !release' > $@ ; \
	   $(MOCKGEN) -package mocks akvorado/conntrackfixer ConntrackConn,DockerClient >> $@ ; \
	fi

console/filter/parser.go: console/filter/parser.peg | $(PIGEON) ; $(info $(M) generate PEG parser for filters…)
	$Q $(PIGEON) -optimize-basic-latin $< > $@

console/frontend/node_modules: console/frontend/package.json console/frontend/package-lock.json
console/frontend/node_modules: ; $(info $(M) fetching node modules…)
	$Q (cd console/frontend ; npm ci --silent --no-audit --no-fund) && touch $@
console/frontend/data/fields.json: console/query_consts.go ; $(info $(M) generate list of selectable fields…)
	$Q sed -En -e 's/^\tqueryColumn([a-zA-Z]+)( .*|$$)/  "\1"/p' $< \
		| sed -E -e '$$ ! s/$$/,/' -e '1s/^ */[/' -e '$$s/$$/]/' > $@
	$Q test -s $@
console/data/frontend: console/frontend/node_modules
console/data/frontend: console/frontend/data/fields.json
console/data/frontend: $(shell $(LSFILES) console/frontend 2> /dev/null)
console/data/frontend: ; $(info $(M) building console frontend…)
	$Q cd console/frontend && npm run --silent build

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

TEST_TARGETS := test-bench test-short test-verbose test-race
.PHONY: $(TEST_TARGETS) check test tests
test-bench:   ARGS=-run=__absolutelynothing__ -bench=. ## Run benchmarks
test-short:   ARGS=-short        ## Run only short tests
test-verbose: ARGS=-v            ## Run tests in verbose mode with coverage reporting
test-race:    ARGS=-race         ## Run tests with race detector
$(TEST_TARGETS): NAME=$(MAKECMDGOALS:test-%=%)
$(TEST_TARGETS): test
check test tests: fmt lint $(GENERATED) | $(GOTESTSUM) ; $(info $(M) running $(NAME:%=% )tests…) @ ## Run tests
	$Q mkdir -p test
	$Q $(GOTESTSUM) --junitfile test/tests.xml -- \
		-timeout $(TIMEOUT)s \
		$(ARGS) $(PKGS)

COVERAGE_MODE = atomic
.PHONY: test-coverage test-coverage-xml test-coverage-lcov
test-coverage: fmt lint $(GENERATED)
test-coverage: | $(GOTESTSUM) ; $(info $(M) running coverage tests…) @ ## Run coverage tests
	$Q mkdir -p test
	$Q $(GOTESTSUM) -- \
		-coverpkg=$(shell echo $(PKGS) | tr ' ' ',') \
		-covermode=$(COVERAGE_MODE) \
		-coverprofile=test/profile.out.tmp $(PKGS)
	$Q GENERATED=$$(awk -F: '(NR > 1) {print $$1}' test/profile.out.tmp \
			| sort | uniq | sed "s+^$(MODULE)/++" \
			| xargs grep -l "^//.*DO NOT EDIT\.$$" \
			| sed "s+\(.*\)+^$(MODULE)/\1:+" | paste -sd '|') ; \
	   if [ -n "$$GENERATED" ]; then grep -Ev "$$GENERATED" test/profile.out.tmp > test/profile.out ; \
	   else cp test/profile.out.tmp test/profile.out ; \
	   fi
	$Q $(GO) tool cover -html=test/profile.out -o test/coverage.html
test-coverage-xml: test-coverage | $(GOCOV) $(GOCOVXML)
	$Q $(GOCOV) convert test/profile.out | $(GOCOVXML) > test/coverage.xml
	@echo -n "Code coverage: "; \
		echo "scale=1;$$(sed -En 's/^<coverage line-rate="([0-9.]+)".*/\1/p' test/coverage.xml) * 100 / 1" | bc -q
test-coverage-lcov: test-coverage | $(GCOV2LCOV)
	$Q $(GCOV2LCOV) -infile test/profile.out -outfile test/coverage.lcov

.PHONY: lint
lint: .lint-go~ .lint-js~ ## Run linting
.lint-go~: $(shell $(LSFILES) '*.go' 2> /dev/null) | $(REVIVE) ; $(info $(M) running golint…)
	$Q $(REVIVE) -formatter friendly -set_exit_status ./...
	$Q touch $@
.lint-js~: $(shell $(LSFILES) '*.js' '*.vue' '*.html' 2> /dev/null)
.lint-js~: console/frontend/node_modules ; $(info $(M) running jslint…)
	$Q cd console/frontend && npm run --silent lint
	$Q touch $@

.PHONY: fmt
fmt: .fmt-go~ .fmt-js~ ## Format all source files
.fmt-go~: $(shell $(LSFILES) '*.go' 2> /dev/null) | $(GOIMPORTS) ; $(info $(M) formatting Go code…)
	$Q $(GOIMPORTS) -local $(MODULE) -w $? < /dev/null
	$Q touch $@
.fmt-js~: $(shell $(LSFILES) '*.js' '*.vue' '*.html' 2> /dev/null)
.fmt-js~: console/frontend/node_modules ; $(info $(M) formatting JS code…)
	$Q cd console/frontend && npm run --silent format
	$Q touch $@

# Misc

.PHONY: licensecheck
licensecheck: console/frontend/node_modules | $(WWHRD) ; $(info $(M) check dependency licenses…) @ ## Check licenses
	$Q err=0 ; go mod vendor && $(WWHRD) --quiet check || err=$$? ; rm -rf vendor/ ; exit $$err
	$Q cd console/frontend ; npm exec --no -- license-compliance \
		--production \
		--allow "MIT;ISC;Apache-2.0;BSD-3-Clause;WTFPL;0BSD" \
		--report detailed

.PHONY: clean
clean: ; $(info $(M) cleaning…)	@ ## Cleanup everything
	@rm -rf $(BIN) test $(GENERATED) inlet/flow/decoder/flow-*.pb.go *~

.PHONY: help
help:
	@grep -hE '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-17s\033[0m %s\n", $$1, $$2}'

.PHONY: version
version:
	@echo $(VERSION)
