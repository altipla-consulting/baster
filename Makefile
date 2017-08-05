
DC = docker-compose
DC_SHELL = @$(DC) run --rm shell
FILES = $(shell find . -not -path "*vendor*" -type f -name '*.go')

build:
	$(DC) build

serve:
	@$(DC) up serve

shell:
	@$(DC_SHELL)

gofmt:
	@gofmt -w $(FILES)
	@gofmt -r '&α{} -> new(α)' -w $(FILES)

godoc:
	@$(DC) run --rm --service-ports godoc
