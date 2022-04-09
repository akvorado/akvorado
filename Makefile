MODULE   = $(shell env GO111MODULE=on $(GO) list -m)
DATE    ?= $(shell date +%FT%T%z)
VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || \
			cat .version 2> /dev/null || echo v0)
PKGS     = $(or $(PKG),$(shell env GO111MODULE=on $(GO) list ./...))
BIN      = bin

GO      = go
TIMEOUT = 15
V = 0
Q = $(if $(filter 1,$V),,@)
M = $(shell if [ "$$(tput colors 2> /dev/null || echo 0)" -ge 8 ]; then printf "\033[34;1m▶\033[0m"; else printf "▶"; fi)

export GO111MODULE=on

GENERATED = inlet/flow/decoder/flow-1.pb.go console/data/frontend console/frontend/node_modules

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
	$Q env GOBIN=$(abspath $(BIN)) $(GO) install $(PACKAGE)@latest

REVIVE = $(BIN)/revive
$(BIN)/revive: PACKAGE=github.com/mgechev/revive

GOCOV = $(BIN)/gocov
$(BIN)/gocov: PACKAGE=github.com/axw/gocov/...

GOCOVXML = $(BIN)/gocov-xml
$(BIN)/gocov-xml: PACKAGE=github.com/AlekSi/gocov-xml

GOTESTSUM = $(BIN)/gotestsum
$(BIN)/gotestsum: PACKAGE=gotest.tools/gotestsum

PROTOC = protoc
PROTOC_GEN_GO = $(BIN)/protoc-gen-go
$(BIN)/protoc-gen-go: PACKAGE=google.golang.org/protobuf/cmd/protoc-gen-go

# Generated files

inlet/flow/decoder/%.pb.go: inlet/flow/data/schemas/%.proto | $(PROTOC_GEN_GO) ; $(info $(M) compiling protocol buffers definition…)
	$Q $(PROTOC) -I=. --plugin=$(PROTOC_GEN_GO) --go_out=. --go_opt=module=$(MODULE) $<

console/frontend/node_modules: console/frontend/package.json console/frontend/yarn.lock
console/frontend/node_modules: ; $(info $(M) fetching node modules…)
	$Q yarn install --silent --frozen-lockfile --cwd console/frontend && touch $@
console/data/frontend: Makefile console/frontend/node_modules
console/data/frontend: console/frontend/index.html console/frontend/vite.config.js
console/data/frontend: $(shell find console/frontend/src -type f)
console/data/frontend: ; $(info $(M) building console frontend…)
	$Q cd console/frontend && yarn --silent build

# These files are versioned in Git, but we may want to update them.
clickhouse/data/protocols.csv:
	$ curl -sL http://www.iana.org/assignments/protocol-numbers/protocol-numbers-1.csv \
		| sed -nE -e "1 s/.*/proto,name,description/p" -e "2,$ s/^([0-9]+,[^ ,]+,[^\",]+),.*/\1/p" \
		> $@
clickhouse/data/asns.csv:	# Need to pipe MaxMind ASN database in CSV format
	$Q sed -ne 's/^[^,]*,//p' \
		| LC_ALL=C sort -n \
		| uniq \
		| grep -v '^[0-9,]*$' \
		| sed -e '1casn,name' > $@

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
lint: console/frontend/node_modules | $(REVIVE) ; $(info $(M) running golint…) @ ## Run golint
	$Q $(REVIVE) -formatter friendly -set_exit_status ./...
	$Q cd console/frontend && yarn --silent lint

.PHONY: fmt
fmt: console/frontend/node_modules ; $(info $(M) running gofmt…) @ ## Run gofmt on all source files
	$Q $(GO) fmt $(PKGS)
	$Q cd console/frontend && yarn --silent format

# Misc

.PHONY: clean
clean: ; $(info $(M) cleaning…)	@ ## Cleanup everything
	@rm -rf $(BIN) test $(GENERATED)

.PHONY: help
help:
	@grep -hE '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-17s\033[0m %s\n", $$1, $$2}'

.PHONY: version
version:
	@echo $(VERSION)
