
LOCALBIN ?= $(shell pwd)/.bin

$(LOCALBIN):
	mkdir -p .bin

.PHONY: values.schema.json
values.schema.json: .bin/helm-schema
	.bin/helm-schema -r -f values.yaml
	# remove empty required arrays
	jq 'walk(if  type == "object" and has("required") and (.required | type == "array" and length == 0) then del(.required) else . end)' 		values.schema.json > tmp.json && mv tmp.json values.schema.json
	# trim spaces
	jq 'walk(if type == "object" and .description? then	.description |= gsub("^(\\s+)|(\\s+)$$"; "")	else .	end	)' values.schema.json  > tmp.json && mv tmp.json values.schema.json


.PHONY: chart
chart: values.schema.deref.json  README.md

.PHONY: README.md
README.md: .bin/helm-docs
	.bin/helm-docs   -t README.md.tpl --skip-version-footer


EGET=$(LOCALBIN)/eget
EGET_BIN=$(LOCALBIN)
$(EGET): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/zyedidia/eget@v1.3.4

.bin/helm-docs: 	$(EGET)
	$(EGET) -t v1.14.2 norwoodj/helm-docs --upgrade-only --to $(EGET_BIN)

.bin/helm-schema: 	$(EGET)
	$(EGET) -t 0.18.1 dadav/helm-schema --upgrade-only --to $(EGET_BIN)

.phony: values.schema.deref.json
values.schema.deref.json: values.schema.json
	npm i @apidevtools/json-schema-ref-parser
	npx @digitak/esrun  deref.ts


.PHONY: lint
lint: chart
	ct lint --charts .
