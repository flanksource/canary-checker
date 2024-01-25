package sync

import (
	"fmt"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/duty/context"
	"github.com/friendsofgo/errors"
)

func SyncTopology(ctx context.Context, dataFile string, configFiles ...string) error {
	if len(configFiles) == 0 {
		return fmt.Errorf("must specify at least one topology definition")
	}
	for _, configfile := range configFiles {
		configs, err := pkg.ParseSystems(configfile, dataFile)
		if err != nil {
			return errors.Wrapf(err, "could not parse %s", configfile)
		}

		for _, config := range configs {
			if _, history := topology.Run(ctx, config); history.AsError() != nil {
				return history.AsError()
			}
		}
	}
	return nil
}
