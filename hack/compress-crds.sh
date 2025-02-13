#!/bin/bash

# Remove various nested properties within the CRD structure, such as checks, forEach, lookup, and properties
# to reduce the CRD size.

base="config/deploy"

cd $base

yq -s '.spec.names.kind' crd.yaml

IFS=',' read -r -a checks < <(yq e '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties | keys | @csv' Canary.yml )

slim=Canary.slim.yml

cp Canary.yml  $slim

schemaPath=".spec.versions[0].schema.openAPIV3Schema.properties.spec.properties"


for check in "${checks[@]}"; do
	relationships="$schemaPath.$check.items.properties.relationships"
	if [[ "$(yq $relationships $slim)" != "null" ]]; then
		yq "$relationships |= {\"type\": \"object\",\"x-kubernetes-preserve-unknown-fields\": true}" -i $slim
	fi
done

for depreceated in containerd containerdPush docker dockerPush helm namespace pod; do
	yq "del($schemaPath.$depreceated.items.properties.**.description)" -i  $slim

done

for description in icon description name transformDeleteStrategy metrics; do
	yq "del($schemaPath.*.items.properties.$description.description)" -i  $slim
done

mv $slim Canary.yml

yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.checks.items.properties)' Component.yml | \
		yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.forEach.properties)' /dev/stdin | \
		yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.[].items.properties.metrics.items.properties)' /dev/stdin | \
		yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.lookup.properties)' /dev/stdin | \
		yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.properties.items.properties.lookup.properties)' /dev/stdin | \
		yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.components.items.properties.forEach.properties)' /dev/stdin | \
		yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.components.items.properties.lookup.properties)' /dev/stdin | \
		yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.components.items.properties.checks.items.properties.inline.properties)' /dev/stdin | \
		yq ea 'del(.. | select(has("scope")).scope | .. | select(has("description")).description)' /dev/stdin | \
		yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.components.items.properties.properties.items.properties.lookup.properties)' /dev/stdin > Component.slim.yaml
		mv Component.slim.yaml Component.yml


yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.checks.items.properties)' Topology.yml | \
		yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.forEach.properties)' /dev/stdin | \
		yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.[].items.properties.metrics.items.properties)' /dev/stdin | \
		yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.lookup.properties)' /dev/stdin | \
		yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.properties.items.properties.lookup.properties)' /dev/stdin | \
		yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.components.items.properties.forEach.properties)' /dev/stdin | \
		yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.components.items.properties.lookup.properties)' /dev/stdin | \
		yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.components.items.properties.checks.items.properties.inline.properties)' /dev/stdin | \
		yq ea 'del(.. | select(has("scope")).scope | .. | select(has("description")).description)' /dev/stdin | \
		yq ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.components.items.properties.properties.items.properties.lookup.properties)' /dev/stdin > Topology.slim.yaml
		mv Topology.slim.yaml Topology.yml
