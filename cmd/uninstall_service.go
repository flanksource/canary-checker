package cmd

import (
	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

var UninstallService = &cobra.Command{
	Use:   "uninstall-service",
	Short: "Install Canary Checker as a Service",
	Run:   uninstallService,
}

func uninstallService(cmd *cobra.Command, args []string) {
	prg := &program{}
	s, err := service.New(prg, ServiceConfig)
	if err != nil {
		serviceLogger.Error(err) // nolint: errcheck
		return
	}
	serviceLogger, err = s.Logger(nil)
	if err != nil {
		serviceLogger.Error(err) // nolint: errcheck
		return
	}
	err = s.Uninstall()
	if err != nil {
		serviceLogger.Warning(err) // nolint: errcheck
		return
	}

	serviceLogger.Info("Service Uninstalled Successfully.") // nolint: errcheck
}
