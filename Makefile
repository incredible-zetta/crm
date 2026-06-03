.PHONY: fmt check-fmt lint test build check staticcheck

GO_PACKAGES := ./...
GO_FILES := $(shell find . -name '*.go' -not -path './.git/*')
STATICCHECK := $(shell command -v staticcheck 2>/dev/null || echo $$(go env GOPATH)/bin/staticcheck)

fmt:
	gofmt -w $(GO_FILES)

check-fmt:
	@test -z "$$(gofmt -l $(GO_FILES))" || (echo "gofmt needed:"; gofmt -l $(GO_FILES); exit 1)

lint:
	go vet $(GO_PACKAGES)
	@if [ -x "$(STATICCHECK)" ]; then \
		"$(STATICCHECK)" $(GO_PACKAGES); \
	else \
		echo "staticcheck not installed; skipping (install: make staticcheck)"; \
	fi

staticcheck:
	go install honnef.co/go/tools/cmd/staticcheck@latest

build:
	go build $(GO_PACKAGES)

test:
	go test $(GO_PACKAGES)

check: check-fmt lint test build
