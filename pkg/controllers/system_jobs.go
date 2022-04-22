package controllers

import (
	"fmt"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
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
			fmt.Println(system.ID)
			// db.AddSystem()
		}
	}
}
