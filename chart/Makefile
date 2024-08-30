
LOCALBIN ?= $(shell pwd)/.bin

$(LOCALBIN):
	mkdir -p .bin

.PHONY: values.schema.json
values.schema.json: .bin/helm-schema
	.bin/helm-schema -r -f values.yaml


.PHONY: chart
chart: values.schema.deref.json  README.md


.PHONY: README.md
README.md: .bin/helm-docs
	.bin/helm-docs   -t README.md.tpl


.bin/helm-docs:
	test -s $(LOCALBIN)/helm-docs  || \
	GOBIN=$(LOCALBIN) go install github.com/norwoodj/helm-docs/cmd/helm-docs@latest

.bin/helm-schema:
	test -s $(LOCALBIN)/helm-schema  || \
	GOBIN=$(LOCALBIN) go install github.com/dadav/helm-schema/cmd/helm-schema@latest

.phony: values.schema.deref.json
values.schema.deref.json: values.schema.json
	npm i @apidevtools/json-schema-ref-parser
	npx @digitak/esrun  deref.ts
