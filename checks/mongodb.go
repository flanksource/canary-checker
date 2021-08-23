package checks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/kommons"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type MongoDBChecker struct {
	kommons *kommons.Client `yaml:"-" json:"-"`
}

func (c *MongoDBChecker) SetClient(client *kommons.Client) {
	c.kommons = client
}

func (c *MongoDBChecker) Type() string {
	return "mongodb"
}

func (c *MongoDBChecker) Run(canary v1.Canary) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range canary.Spec.MongoDB {
		result := c.Check(canary, conf)
		if result != nil {
			results = append(results, result)
		}
	}
	return results
}

func (c *MongoDBChecker) Check(canary v1.Canary, extConfig external.Check) *pkg.CheckResult {
	start := time.Now()
	check := extConfig.(v1.MongoDBCheck)
	endpoint := getMongoDBEndpoint(check.URL, check.GetPort())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var client *mongo.Client
	var err error
	if check.Credentials != nil {
		auth, err := GetAuthValues(check.Credentials.Authentication, c.kommons, canary.Namespace)
		if err != nil {
			return pkg.Fail(check).ErrorMessage(err).StartTime(start)
		}
		credential := options.Credential{
			AuthMechanism:           check.Credentials.AuthMechanism,
			AuthSource:              check.Credentials.AuthSource,
			AuthMechanismProperties: check.Credentials.AuthMechanismProperties,
			PasswordSet:             check.Credentials.PasswordSet,
			Username:                auth.Username.Value,
			Password:                auth.Password.Value,
		}
		client, err = mongo.Connect(ctx, options.Client().ApplyURI(endpoint).SetAuth(credential))
		if err != nil {
			return pkg.Fail(check).ErrorMessage(err).StartTime(start)
		}
	} else {
		client, err = mongo.Connect(ctx, options.Client().ApplyURI(endpoint))
		if err != nil {
			return pkg.Fail(check).ErrorMessage(err).StartTime(start)
		}
	}

	defer client.Disconnect(ctx) //nolint: errcheck
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		return pkg.Fail(check).ErrorMessage(err).StartTime(start)
	}
	return pkg.Success(check).StartTime(start)
}

func getMongoDBEndpoint(url string, port int) string {
	if strings.HasPrefix(url, "mongodb://") {
		return fmt.Sprintf("%v:%v", url, port)
	}
	return fmt.Sprintf("mongodb://%v:%v", url, port)
}
