VERSION := 2.0.0

GOLANGCI_VERSION := 2.12.2
GOBINDATA_VERSION := 3
GOX_VERSION := 1.0.1

TEST_FLAGS ?=

GO111MODULE := on
export GO111MODULE

.PHONY: all
all: deps assets queries build test lint

.PHONY: build
build: deps assets queries
	go build .
	cd cmd/mhsendmail && go build .

.PHONY: test
test: deps assets
	[ -n "$$TEST_MONGODB_URI" ] || echo 'Warning, MongoDB storage testing disabled!' >&2
	[ -n "$$TEST_POSTGRESQL_URI" ] || echo 'Warning, PostgreSQL storage testing disabled!' >&2
	go test -race $(TEST_FLAGS) ./...

.PHONY: release
release: deps assets test lint
	go install github.com/mitchellh/gox@v${GOX_VERSION}
	gox -ldflags "-X main.version=${VERSION}" -output="build/{{.Dir}}_{{.OS}}_{{.Arch}}" .

.PHONY: lint
lint: deps
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v${GOLANGCI_VERSION}
	golangci-lint run

.PHONY: deps
deps:
	go mod download
	go install github.com/go-bindata/go-bindata/go-bindata@v${GOBINDATA_VERSION}

.PHONY: assets
assets: deps
	rm -f generated/assets/assets.go
	go-bindata -o generated/assets/assets.go -pkg assets assets/...

.PHONY: queries
queries: deps
	rm -f generated/queries/queries.go
	go-bindata -o generated/queries/queries.go -pkg queries queries/...
