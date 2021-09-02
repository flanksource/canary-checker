package checks

import (
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"

	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type MongoDBChecker struct {
}

func (c *MongoDBChecker) Type() string {
	return "mongodb"
}

func (c *MongoDBChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.MongoDB {
		result := c.Check(ctx, conf)
		if result != nil {
			results = append(results, result)
		}
	}
	return results
}

func (c *MongoDBChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	start := time.Now()
	check := extConfig.(v1.MongoDBCheck)
	var client *mongo.Client
	var err error
	if check.Authentication != nil {
		auth, err := GetAuthValues(check.Authentication, ctx.Kommons, ctx.Canary.Namespace)
		if err != nil {
			return pkg.Fail(check).ErrorMessage(err).StartTime(start)
		}
		credential := options.Credential{
			Username: auth.Username.Value,
			Password: auth.Password.Value,
		}
		client, err = mongo.Connect(ctx, options.Client().ApplyURI(check.Connection).SetAuth(credential))
		if err != nil {
			return pkg.Fail(check).ErrorMessage(err).StartTime(start)
		}
	} else {
		client, err = mongo.Connect(ctx, options.Client().ApplyURI(check.Connection))
		if err != nil {
			return pkg.Fail(check).ErrorMessage(err).StartTime(start)
		}
	}

	defer client.Disconnect(ctx) //nolint: errcheck
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		return pkg.Fail(check).ErrorMessage(err).StartTime(start)
	}
	return pkg.Success(check).StartTime(start)
}
