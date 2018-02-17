
FILES = $(shell find . -not -path "*vendor*" -type f -name '*.go')

gofmt:
	@gofmt -w $(FILES)
	@gofmt -r '&α{} -> new(α)' -w $(FILES)

serve:
	actools start baster

bench:
	actools go test -bench . ./pkg/proxy
