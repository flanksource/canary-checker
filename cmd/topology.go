package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/flanksource/commons/timer"
	"github.com/flanksource/duty"

	"github.com/spf13/cobra"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	configSync "github.com/flanksource/canary-checker/pkg/sync"
	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
)

var topologyRunNamespace string

var Topology = &cobra.Command{
	Use: "topology",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		db.ConnectionString = readFromEnv(db.ConnectionString)
		if db.IsConfigured() {
			if err := db.Init(); err != nil {
				logger.Debugf("error connecting with postgres %v", err)
			}
		}
	},
}

var queryParams duty.TopologyOptions
var topologyOutput string

var QueryTopology = &cobra.Command{
	Use:   "query",
	Short: "Query the topology",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		if !db.IsConfigured() {
			logger.Fatalf("Must specify --db or DB_URL env")
		}

		if err := db.Init(); err != nil {
			logger.Fatalf("error connecting with postgres %v", err)
		}

		results, err := topology.Query(queryParams)
		if err != nil {
			logger.Fatalf("Failed to query topology: %v", err)
		}
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
	},
}

var AddTopology = &cobra.Command{
	Use:   "topology <system.yaml>",
	Short: "Add a new topology spec",
	Run: func(cmd *cobra.Command, configFiles []string) {
		opts := getTopologyRunOptions(0)
		if err := configSync.SyncTopology(opts, dataFile, configFiles...); err != nil {
			logger.Fatalf("Could not sync topology: %v", err)
		}
	},
}

func getTopologyRunOptions(depth int) topology.TopologyRunOptions {
	logger.Tracef("depth: %v", depth)
	kommonsClient, err := pkg.NewKommonsClient()
	if err != nil {
		logger.Warnf("Failed to get kommons client, features that read kubernetes configs will fail: %v", err)
	}
	return topology.TopologyRunOptions{
		Client:    kommonsClient,
		Depth:     10,
		Namespace: topologyRunNamespace,
	}
}

// StaticTemplatedID for topologies created by CLI, to ensure that components are updated rather than duplicated
var StaticTemplatedID string = "cf38821d-434f-4496-9bbd-c0c633bb2699"

var RunTopology = &cobra.Command{
	Use:   "run <system.yaml>",
	Short: "Execute topology and return",
	Run: func(cmd *cobra.Command, configFiles []string) {
		timer := timer.NewTimer()
		if len(configFiles) == 0 {
			log.Fatalln("Must specify at least one topology definition")
		}

		opts := getTopologyRunOptions(10)

		var results = []*pkg.Component{}

		wg := sync.WaitGroup{}

		for _, configfile := range configFiles {
			configs, err := pkg.ParseSystems(configfile, dataFile)
			if err != nil {
				logger.Errorf("Could not parse %s: %v", configfile, err)
				continue
			}
			logger.Infof("Checking %s, %d systems found", configfile, len(configs))
			for _, config := range configs {
				wg.Add(1)
				_config := config
				if _config.GetPersistedID() == "" {
					_config.Status = v1.TopologyStatus{
						PersistedID: &StaticTemplatedID,
					}
				}
				go func() {
					components := topology.Run(opts, _config)
					results = append(results, components...)
					wg.Done()
				}()
			}
		}
		wg.Wait()

		logger.Infof("Checked %d systems in %v", len(results), timer)

		if db.IsConnected() {
			if err := db.PersistComponents(results); err != nil {
				logger.Errorf("error persisting results: %v", err)
			}
		}

		if topologyOutput != "" {
			data, _ := json.Marshal(results)
			logger.Infof("Writing results to %s", topologyOutput)
			if err := os.WriteFile(topologyOutput, data, 0644); err != nil {
				log.Fatalln(err)
			}
		}

	},
}

func init() {
	QueryTopology.Flags().StringVar(&queryParams.ID, "component", "", "The component id to query")
	QueryTopology.Flags().IntVar(&queryParams.Depth, "depth", 1, "The depth of the components to return")
	RunTopology.Flags().StringVarP(&topologyOutput, "output", "o", "", "Output file to write results to")
	RunTopology.Flags().StringVarP(&topologyRunNamespace, "namespace", "n", "default", "Namespace to query")
	Topology.AddCommand(RunTopology, QueryTopology, AddTopology)
	Root.AddCommand(Topology)
}
