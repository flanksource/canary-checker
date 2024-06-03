package main

import (
	"fmt"
	"os"
	"path"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/schema/openapi"
	"github.com/spf13/cobra"
)

var schemas = map[string]interface{}{
	"canary":    &v1.Canary{},
	"topology":  &v1.Topology{},
	"component": &v1.Component{},
}

var generateSchema = &cobra.Command{
	Use: "generate-schema",
	Run: func(cmd *cobra.Command, args []string) {
		for file, obj := range schemas {
			p := path.Join(schemaPath, file+".schema.json")
			if err := openapi.WriteSchemaToFile(p, obj); err != nil {
				logger.Fatalf("unable to save schema: %v", err)
			}
			logger.Infof("Saved OpenAPI schema to %s", p)
		}

		for _, check := range v1.AllChecks {
			p := path.Join(schemaPath, fmt.Sprintf("health_%s.schema.json", check.GetType()))
			if err := openapi.WriteSchemaToFile(p, check); err != nil {
				logger.Fatalf("unable to save schema: %v", err)
			}

			logger.Infof("Saved OpenAPI schema to %s", p)
		}
	},
}

var schemaPath string

func main() {
	generateSchema.Flags().StringVar(&schemaPath, "schema-path", "../../config/schemas", "Path to save JSON schema to")
	if err := generateSchema.Execute(); err != nil {
		os.Exit(1)
	}
}
