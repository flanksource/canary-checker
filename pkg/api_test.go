package pkg

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestPostgresYaml(t *testing.T) {
	var yamlTests = []struct {
		description string
		yamlFixture string
		wantConfig  Config
	}{
		{
			"we can parse arbitrary key value results",
			"../fixtures/postgres_yaml_arbitrary_results.yaml",
			Config{
				Postgres: []Postgres{
					{
						PostgresCheck{
							Driver:     "someDriver",
							Connection: "someConnection",
							Query:      "someQuery",
							Results: PostgresResults{
								map[string]string{
									"columnA": "valueA",
									"columnB": "valueB",
									"columnC": "valueC",
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range yamlTests {
		t.Run(tt.description, func(t *testing.T) {

			gotConfig := ParseConfig(tt.yamlFixture)

			//using cmpopts.EquateEmpty()
			//to consider maps and slices with a length of zero to be equal,
			//regardless of whether they are nil.
			//for some reason S3Check deserialise to empty rather than nil
			if !cmp.Equal(tt.wantConfig, gotConfig, cmpopts.EquateEmpty()) {
				t.Errorf("want %v, got %v", tt.wantConfig, gotConfig)
			}
		})

	}

}
