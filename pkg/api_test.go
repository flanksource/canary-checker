package pkg

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestPostgresYaml(t *testing.T) {
	//assignment helper
	c := func(s int) *int {
		return &s
	}
	var yamlTests = []struct {
		description  string
		yamlFixture  string
		error        bool
		errorMessage string
		wantConfig   Config
	}{
		{
			description:  "we can parse arbitrary key value results for postgres",
			yamlFixture:  "../fixtures/postgres_yaml_arbitrary_results.yaml",
			error:        false,
			errorMessage: "",
			wantConfig: Config{
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
		{
			description:  "we can parse single value result for postgres",
			yamlFixture:  "../fixtures/postgres_yaml_single_result.yaml",
			error:        false,
			errorMessage: "",
			wantConfig: Config{
				Postgres: []Postgres{
					{
						PostgresCheck{
							Driver:     "someDriver",
							Connection: "someConnection",
							Query:      "someQuery",
							Result:     c(1),
						},
					},
				},
			},
		},
		{
			description:  "if no driver is supplied a default of 'postgres' is used",
			yamlFixture:  "../fixtures/postgres_yaml_default_driver.yaml",
			error:        false,
			errorMessage: "",
			wantConfig: Config{
				Postgres: []Postgres{
					{
						PostgresCheck{
							Driver:     "postgres",
							Connection: "someConnection",
							Query:      "someQuery",
							Result:     c(1),
						},
					},
				},
			},
		},
		{
			"we can't parse a config with `result` and `results`",
			"../fixtures/postgres_yaml_invalid_results_and_result.yaml",
			true,
			"Invalid postgres config: can't specify single AND compound result!",
			Config{},
		},
	}
	for _, tt := range yamlTests {
		t.Run(tt.description, func(t *testing.T) {

			gotConfig, err := ParseConfig(tt.yamlFixture)

			if err != nil {
				if tt.error != true {
					//we didn't expect an error!
					t.Errorf("Unexpected Error parsing config: (%v)", err)
				} else {
					//error was expected
					t.Logf("Expected Error parsing config: (%v)", err)
				}
			}

			//using cmpopts.EquateEmpty()
			//to consider maps and slices with a length of zero to be equal,
			//regardless of whether they are nil.
			//for some reason S3Check deserialise to empty rather than nil
			if !cmp.Equal(tt.wantConfig, gotConfig, cmpopts.EquateEmpty()) {
				t.Errorf("Test '%s': want %v, got %v", tt.description, tt.wantConfig, gotConfig)
			}
		})

	}

}
