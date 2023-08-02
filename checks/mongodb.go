package checks

import (
	gocontext "context"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"

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

func (c *MongoDBChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.MongoDB {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *MongoDBChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.MongoDBCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)
	var err error

	connection, err := ctx.GetConnection(check.Connection)
	if err != nil {
		return results.Failf("error getting connection: %v", err)
	}

	opts := options.Client().
		ApplyURI(connection.URL).
		SetConnectTimeout(3 * time.Second).
		SetSocketTimeout(3 * time.Second)

	_ctx, cancel := gocontext.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	client, err := mongo.Connect(_ctx, opts)
	if err != nil {
		return results.ErrorMessage(err)
	}
	defer client.Disconnect(ctx) //nolint: errcheck

	err = client.Ping(_ctx, readpref.Primary())
	if err != nil {
		return results.ErrorMessage(err)
	}

	return results
}
