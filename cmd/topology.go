package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"sync"

	"github.com/flanksource/commons/timer"

	"github.com/spf13/cobra"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
)

var Topology = &cobra.Command{
	Use: "topology",
}

var QueryTopology = &cobra.Command{
	Use:   "query",
	Short: "Query the topology",
	Run: func(cmd *cobra.Command, args []string) {
		if !db.IsConfigured() {
			logger.Fatalf("Must specify --db or DB_URL env")
		}

		if err := db.Init(db.ConnectionString); err != nil {
			logger.Fatalf("error connecting with postgres %v", err)
		}

		results, err := topology.Query(topology.TopologyParams{})
		if err != nil {
			logger.Fatalf("Failed to query topology: %v", err)
		}
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
	},
}

var RunTopology = &cobra.Command{
	Use:   "run <system.yaml>",
	Short: "Execute checks and return",
	Run: func(cmd *cobra.Command, configFiles []string) {
		timer := timer.NewTimer()
		if len(configFiles) == 0 {
			log.Fatalln("Must specify at least one canary")
		}
		kommonsClient, err := pkg.NewKommonsClient()
		if err != nil {
			logger.Warnf("Failed to get kommons client, features that read kubernetes configs will fail: %v", err)
		}
		var results = []*pkg.System{}

		wg := sync.WaitGroup{}

		for _, configfile := range configFiles {
			configs, err := pkg.ParseSystems(configfile, dataFile)
			if err != nil {
				logger.Errorf("Could not parse %s: %v", configfile, err)
				continue
			}
			logger.Infof("Checking %s, %d systems found", configfile, len(configs))
			for _, config := range configs {
				if namespace != "" {
					config.Namespace = namespace
				}
				if config.Name == "" {
					config.Name = CleanupFilename(configfile)
				}
				wg.Add(1)
				_config := config
				go func() {
					systems := topology.Run(kommonsClient, _config)
					results = append(results, systems...)
					wg.Done()
				}()
			}
		}
		wg.Wait()

		logger.Infof("Checked %d systems in %v", len(results), timer)

		if db.IsConfigured() {
			if err := db.Init(db.ConnectionString); err != nil {
				logger.Debugf("error connecting with postgres, exporting to disk: %v", err)
			} else {
				if err := db.Persist(results); err != nil {
					logger.Errorf("error persisting results: %v", err)
				}
				return
			}
		}

		data, _ := json.Marshal(results)
		if outputFile == "" {
			outputFile = "topology.json"
		}
		logger.Infof("Writing results to %s", outputFile)
		if err := ioutil.WriteFile(outputFile, data, 0644); err != nil {
			log.Fatalln(err)
		}

	},
}

func init() {
	Topology.AddCommand(RunTopology, QueryTopology)
	Root.AddCommand(Topology)
}
