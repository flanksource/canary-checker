package controllers

import (
	"fmt"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/db/types"
	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
	"github.com/google/uuid"
)

func SyncSystemsJobs() {
	if Kommons == nil {
		var err error
		Kommons, err = pkg.NewKommonsClient()
		if err != nil {
			logger.Warnf("Failed to get kommons client, features that read kubernetes config will fail: %v", err)
		}
	}

	systemTemplates, err := db.GetAllSystemTemplates()
	if err != nil {
		logger.Errorf("Failed to get systemTemplate: %v", err)
		return
	}

	for _, systemTemplate := range systemTemplates {
		opts := topology.TopologyRunOptions{
			Client:    Kommons,
			Depth:     10,
			Namespace: systemTemplate.Namespace,
		}
		systems := topology.Run(opts, systemTemplate)
		for _, system := range systems {
			// fmt.Println(*systemTemplate.Status.PersistentID)
			fmt.Println(system.Properties)
			system.Name = systemTemplate.Name
			system.Namespace = systemTemplate.Namespace
			system.Labels = types.JSONStringMap(systemTemplate.Labels)
			system.SystemTemplateID = uuid.MustParse(*systemTemplate.Status.PersistentID)
			err = db.PersistSystem(system)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}
