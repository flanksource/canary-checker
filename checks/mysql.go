package checks

import (
	"github.com/flanksource/canary-checker/api/context"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	_ "github.com/go-sql-driver/mysql" // Necessary for mysql
)

func init() {
	//register metrics here
}

type MysqlChecker struct{}

// Type: returns checker type
func (c *MysqlChecker) Type() string {
	return "mysql"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *MysqlChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.Mysql {
		results = append(results, c.Check(ctx, conf))
	}
	return results
}

func (c *MysqlChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	return CheckSQL(ctx, extConfig.(v1.MysqlCheck).SQLCheck)
}
