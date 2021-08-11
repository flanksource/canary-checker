package checks

import (
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
)

func RunChecks(canary v1.Canary) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	kommonsClient, err := pkg.NewKommonsClient()
	if err != nil {
		logger.Warnf("Failed to get kommons client, features that read kubernetes configs will fail: %v", err)
	}

	canary.Spec.SetSQLDrivers()
	for _, c := range All {
		switch cs := c.(type) {
		case SetsClient:
			cs.SetClient(kommonsClient)
		}
		result := c.Run(canary)
		for _, r := range result {
			if r != nil {
				results = append(results, r)
			}
		}
	}
	return results
}

