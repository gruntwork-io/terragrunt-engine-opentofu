
GOFMT_FILES?=$$(find . -name '*.go')

default: build

build: $(shell find . \( -type f -name '*.go' -print \))
	set -xe ;\
	go build -o engine -ldflags "-extldflags '-static'" .

clean:
	rm -f engine

lint:
	golangci-lint run ./...

test:
	go test ./...

tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.54.2

fmt:
	@echo "Running source files through gofmt..."
	gofmt -w $(GOFMT_FILES)


.PHONY: default lint protoc test tools
