
GOFMT_FILES?=$$(find . -name '*.go')

default: build

build: $(shell find . \( -type f -name '*.go' -print \))
	set -xe ;\
	vtag=$$(git describe --tags --abbrev=12 --dirty --broken) ;\
	go build -o terragrunt-iac-engine-opentofu -ldflags "-X github.com/gruntwork-io/go-commons/version.Version=$${vtag} -extldflags '-static'" .

clean:
	rm -f engine

lint: SHELL:=/bin/bash
lint:
	golangci-lint run -c <(curl -s https://raw.githubusercontent.com/gruntwork-io/terragrunt/main/.golangci.yml) ./...

update-local-lint: SHELL:=/bin/bash
update-local-lint:
	curl -s https://raw.githubusercontent.com/gruntwork-io/terragrunt/main/.golangci.yml --output .golangci.yml
	tmpfile=$$(mktemp) ;\
	echo '# This file is generated using `make update-local-lint` to track the linting used in Terragrunt. Do not edit manually.' | cat - .golangci.yml > $${tmpfile} && mv $${tmpfile} .golangci.yml

test:
	go test -v ./...

tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.1.6

fmt:
	@echo "Running source files through gofmt..."
	gofmt -w $(GOFMT_FILES)

pre-commit:
	pre-commit run --all-files

.PHONY: tools pre-commit lint protoc test default
