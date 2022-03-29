package sync

import (
	"fmt"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
	"github.com/friendsofgo/errors"
)

func SyncTopology(opts topology.TopologyRunOptions, dataFile string, configFiles ...string) error {
	if len(configFiles) == 0 {
		return fmt.Errorf("must specify at least one topology definition")
	}
	for _, configfile := range configFiles {
		configs, err := pkg.ParseSystems(configfile, dataFile)
		if err != nil {
			return errors.Wrapf(err, "could not parse %s", configfile)
		}

		for _, config := range configs {
			systems := topology.Run(opts, config)
			for _, system := range systems {
				if id, err := db.AddSystemSpec(system.Id, config); err != nil {
					return errors.Wrapf(err, "could not add %s", configfile)
				} else {
					logger.Infof("Added %s: %s", configfile, id)
				}
			}
		}
	}
	return nil
}
