
FILES = $(shell find . -not -path "*vendor*" -type f -name '*.go')

.PHONY: deploy

gofmt:
	@gofmt -w $(FILES)
	@gofmt -r '&α{} -> new(α)' -w $(FILES)

serve:
	actools start baster

bench:
	actools go test -bench . ./pkg/proxy

deploy:
ifndef tag
	$(error tag is not set)
endif

	git push
	actools go build -o baster github.com/altipla-consulting/baster/cmd/baster
	docker build -t altipla/baster:latest .
	docker build -t altipla/baster:$(tag) .
	rm baster
	docker push altipla/baster:latest
	docker push altipla/baster:$(tag)
	git tag $(tag)
	git push origin --tags
