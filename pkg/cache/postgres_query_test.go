package cache

import (
	"fmt"
	"testing"

	"github.com/flanksource/canary-checker/pkg/db"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var cases = []struct {
	fixture   QueryParams
	returnErr bool
	args      map[string]interface{}
	clause    string
}{
	{
		fixture: QueryParams{
			Start:       "1h",
			Trace:       true,
			StatusCount: 5,
		},
		args: map[string]interface{}{
			"start": float64(60),
		},
		returnErr: false,
		clause:    "time > (NOW() - Interval '1 minute' * :start)",
	},
	// {
	// 	fixture: QueryParams{
	// 		End: "1h",
	// 	},
	// 	args: map[string]interface{}{
	// 		"end": float64(60),
	// 	},
	// 	returnErr: false,
	// 	clause:    "time < (NOW() - Interval '1 minute' * :end)",
	// },
	// {
	// 	fixture: QueryParams{
	// 		Start: "2h",
	// 		End:   "1h",
	// 	},
	// 	args: map[string]interface{}{
	// 		"start": float64(120),
	// 		"end":   float64(60),
	// 	},
	// 	returnErr: false,
	// 	clause:    "time BETWEEN (NOW() - Interval '1 minute' * :start) AND (NOW() - Interval '1 minute' * :end)",
	// },
}

func TestQueries(t *testing.T) {
	if err := db.Init("postgres://root@localhost:5432/canary_checker?sslmode=disable"); err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}
	psql := NewPostgresCache(db.Pool)
	for _, cache := range []Cache{psql, &cacheChain{
		Chain: []Cache{
			InMemoryCache,
			psql,
		}}} {
		_cache := cache
		t.Run(fmt.Sprintf("%T", cache), func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.fixture.String(), func(t *testing.T) {
					results, err := _cache.Query(tc.fixture)
					if err != nil {
						t.Errorf("Expected no error, got: %v", err)
					}
					g := NewWithT(t)
					g.Expect(len(results)).To(BeNumerically(">", 1))
					check := results[0]
					t.Log(*check)
					g.Expect(*check).To((MatchFields(IgnoreExtras, Fields{
						"Name":       Not(BeEmpty()),
						"Namespace":  Not(BeEmpty()),
						"Type":       Not(BeEmpty()),
						"Key":        Not(BeEmpty()),
						"RunnerName": Not(BeEmpty()),
						"Statuses":   HaveLen(tc.fixture.StatusCount),
					})))

				})
			}
		})
	}
}

func TestDurations(t *testing.T) {
	for _, tc := range cases {
		t.Run(tc.fixture.String(), func(t *testing.T) {
			clause, args, err := tc.fixture.GetWhereClause()
			returnedErr := err != nil
			g := NewWithT(t)
			g.Expect(returnedErr).To(Equal(tc.returnErr))
			g.Expect(args).To(Equal(tc.args))
			g.Expect(clause).To(Equal(tc.clause))
		})
	}
}
