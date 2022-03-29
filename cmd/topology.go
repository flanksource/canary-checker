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
	configSync "github.com/flanksource/canary-checker/pkg/sync"
	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
)

var Topology = &cobra.Command{
	Use: "topology",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if db.IsConfigured() {
			if err := db.Init(); err != nil {
				logger.Debugf("error connecting with postgres %v", err)
			}
		}
	},
}

var queryParams topology.TopologyParams
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
		Namespace: namespace,
	}
}

var RunTopology = &cobra.Command{
	Use:   "run <system.yaml>",
	Short: "Execute topology and return",
	Run: func(cmd *cobra.Command, configFiles []string) {
		timer := timer.NewTimer()
		if len(configFiles) == 0 {
			log.Fatalln("Must specify at least one topology definition")
		}
		opts := getTopologyRunOptions(10)

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
				wg.Add(1)
				_config := config
				go func() {
					systems := topology.Run(opts, _config)
					results = append(results, systems...)
					wg.Done()
				}()
			}
		}
		wg.Wait()

		logger.Infof("Checked %d systems in %v", len(results), timer)

		if db.IsConnected() {
			if err := db.Persist(results); err != nil {
				logger.Errorf("error persisting results: %v", err)
			}
			return
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
	Topology.PersistentFlags().StringVarP(&namespace, "namespace", "n", "default", "Namespace to query")
	QueryTopology.Flags().StringVar(&queryParams.TopologyId, "topology", "", "The topology id to query")
	QueryTopology.Flags().StringVar(&queryParams.ComponentId, "component", "", "The component id to query")
	QueryTopology.Flags().IntVar(&queryParams.Depth, "depth", 3, "The depth of the components to return")

	Topology.AddCommand(RunTopology, QueryTopology, AddTopology)
	Root.AddCommand(Topology)
}
