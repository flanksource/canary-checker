package topology

import (
	"testing"

	embeddedPG "github.com/fergusstrange/embedded-postgres"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/testutils"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	postgresServer *embeddedPG.EmbeddedPostgres
)

func TestTopologySync(t *testing.T) {
	RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Test topology sync runs")
}

var _ = ginkgo.BeforeSuite(func() {
	var err error

	port := 9842
	config, dbString := testutils.GetEmbeddedPGConfig("test_topology", port)
	postgresServer = embeddedPG.NewDatabase(config)
	if err = postgresServer.Start(); err != nil {
		ginkgo.Fail(err.Error())
	}
	logger.Infof("Started postgres on port: %d", port)

	if db.Gorm, db.Pool, err = duty.SetupDB(dbString, nil); err != nil {
		ginkgo.Fail(err.Error())
	}
})

var _ = ginkgo.AfterSuite(func() {
	logger.Infof("Stopping postgres")
	if err := postgresServer.Stop(); err != nil {
		ginkgo.Fail(err.Error())
	}
})
