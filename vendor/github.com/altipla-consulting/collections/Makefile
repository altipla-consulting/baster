
GO_FILES = $(shell find . -type f -name '*.go' -not -path './vendor/*')

gofmt:
	@gofmt -w $(GO_FILES)
	@gofmt -r '&α{} -> new(α)' -w $(GO_FILES)
