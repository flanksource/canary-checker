package sync

import (
	"path"

	"github.com/pkg/errors"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/context"
)

func SyncCanary(ctx context.Context, dataFile string, configFiles ...string) error {
	if len(configFiles) == 0 {
		return errors.New("No config file specified, running in read-only mode")
	}
	for _, configfile := range configFiles {
		logger.Infof("Syncing canary config %s", configfile)
		configs, err := pkg.ParseConfig(configfile, dataFile)
		if err != nil {
			return errors.Wrapf(err, "could not parse %s", configfile)
		}

		for _, canary := range configs {
			_, err := db.PersistCanary(ctx, canary, path.Base(configfile))
			if err != nil {
				return err
			}
		}
	}
	return nil
}
