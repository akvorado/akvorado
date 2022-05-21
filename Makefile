MODULE   = $(shell env GO111MODULE=on $(GO) list -m)
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

export GO111MODULE=on

GENERATED = \
	inlet/flow/decoder/flow-1.pb.go \
	common/clickhousedb/mocks/mock_driver.go \
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
$(BIN)/gocov: PACKAGE=github.com/axw/gocov/gocov@latest

GOCOVXML = $(BIN)/gocov-xml
$(BIN)/gocov-xml: PACKAGE=github.com/AlekSi/gocov-xml@latest

GOTESTSUM = $(BIN)/gotestsum
$(BIN)/gotestsum: PACKAGE=gotest.tools/gotestsum@latest

MOCKGEN = $(BIN)/mockgen
$(BIN)/mockgen: PACKAGE=github.com/golang/mock/mockgen@v1.6.0

PROTOC = protoc
PROTOC_GEN_GO = $(BIN)/protoc-gen-go
$(BIN)/protoc-gen-go: PACKAGE=google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.0

PIGEON = $(BIN)/pigeon
$(BIN)/pigeon: PACKAGE=github.com/mna/pigeon@v1.1.0

# Generated files

.DELETE_ON_ERROR:

inlet/flow/decoder/%.pb.go: inlet/flow/data/schemas/%.proto | $(PROTOC_GEN_GO) ; $(info $(M) compiling protocol buffers definition…)
	$Q $(PROTOC) -I=. --plugin=$(PROTOC_GEN_GO) --go_out=. --go_opt=module=$(MODULE) $<

common/clickhousedb/mocks/mock_driver.go: Makefile | $(MOCKGEN) ; $(info $(M) generate mocks for ClickHouse driver…)
	$Q $(MOCKGEN) -destination $@ -package mocks \
		github.com/ClickHouse/clickhouse-go/v2/lib/driver Conn,Row,Rows,ColumnType
	$Q sed -i'' -e '1i //go:build !release' $@

console/filter/parser.go: console/filter/parser.peg | $(PIGEON) ; $(info $(M) generate PEG parser for filters…)
	$Q $(PIGEON) -optimize-basic-latin $< > $@

console/frontend/node_modules: console/frontend/package.json console/frontend/yarn.lock
console/frontend/node_modules: ; $(info $(M) fetching node modules…)
	$Q yarn install --silent --frozen-lockfile --cwd console/frontend && touch $@
console/frontend/data/fields.json: console/query.go ; $(info $(M) generate list of selectable fields…)
	$Q sed -En -e 's/^\tqueryColumn([a-zA-Z]+)( .*|$$)/  "\1"/p' $< \
		| sed -E -e '1i [' -e '$$ ! s/$$/,/' -e '$$a ]'> $@
	$Q test -s $@
console/data/frontend: Makefile console/frontend/node_modules
console/data/frontend: console/frontend/index.html console/frontend/vite.config.js
console/data/frontend: console/frontend/data/fields.json
console/data/frontend: $(shell $(LSFILES) console/frontend/src 2> /dev/null)
console/data/frontend: ; $(info $(M) building console frontend…)
	$Q cd console/frontend && yarn --silent build

orchestrator/clickhouse/data/asns.csv: ; $(info $(M) generate ASN map…)
	$Q curl -sL https://vincentbernat.github.io/asn2org/asns.csv | sed 's|,[^,]*$$||' > $@
	$Q test -s $@
orchestrator/clickhouse/data/protocols.csv: # We keep this one in Git
	$Q curl -sL http://www.iana.org/assignments/protocol-numbers/protocol-numbers-1.csv \
		| sed -nE -e "1 s/.*/proto,name,description/p" -e "2,$ s/^([0-9]+,[^ ,]+,[^\",]+),.*/\1/p" \
		> $@
	$Q test -s $@

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
	$Q $(GOTESTSUM) --junitfile test/tests.xml -- -timeout $(TIMEOUT)s $(ARGS) $(PKGS)

COVERAGE_MODE = atomic
.PHONY: test-coverage
test-coverage: fmt lint $(GENERATED)
test-coverage: | $(GOCOV) $(GOCOVXML) $(GOTESTSUM) ; $(info $(M) running coverage tests…) @ ## Run coverage tests
	$Q mkdir -p test
	$Q $(GOTESTSUM) -- \
		-coverpkg=$(shell echo $(PKGS) | tr ' ' ',') \
		-covermode=$(COVERAGE_MODE) \
		-coverprofile=test/profile.out $(PKGS)
	$Q $(GO) tool cover -html=test/profile.out -o test/coverage.html
	$Q $(GOCOV) convert test/profile.out | $(GOCOVXML) > test/coverage.xml
	@echo -n "Code coverage: "; \
		echo "scale=1;$$(sed -En 's/^<coverage line-rate="([0-9.]+)".*/\1/p' test/coverage.xml) * 100 / 1" | bc -q

.PHONY: lint
lint: .lint-go~ .lint-js~ ; $(info $(M) running lint…) @ ## Run linting
.lint-go~: $(shell $(LSFILES) '*.go' 2> /dev/null) | $(REVIVE)
	$Q $(REVIVE) -formatter friendly -set_exit_status $?
	$Q touch $@
.lint-js~: console/frontend/node_modules
	$Q cd console/frontend && yarn --silent lint
	$Q touch $@

.PHONY: fmt
fmt: .fmt-go~ .fmt-js~ ; $(info $(M) formatting code…) @ ## Format all source files
.fmt-go~: $(shell $(LSFILES) '*.go' 2> /dev/null) | $(GOIMPORTS)
	$Q $(GOIMPORTS) -local $(MODULE) -w $? < /dev/null
	$Q touch $@
.fmt-js~: console/frontend/node_modules
	$Q cd console/frontend && yarn --silent format
	$Q touch $@

# Misc

.PHONY: clean
clean: ; $(info $(M) cleaning…)	@ ## Cleanup everything
	@rm -rf $(BIN) test $(GENERATED) *~

.PHONY: help
help:
	@grep -hE '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-17s\033[0m %s\n", $$1, $$2}'

.PHONY: version
version:
	@echo $(VERSION)
