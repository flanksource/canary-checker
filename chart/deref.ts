import $RefParser from "@apidevtools/json-schema-ref-parser";
import fs from "fs";


let schema = JSON.parse(fs.readFileSync('values.schema.json', 'utf8'));
await $RefParser.dereference(schema)

fs.writeFileSync('values.schema.deref.json', JSON.stringify(schema, null, 2));

