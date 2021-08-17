package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/flanksource/canary-checker/pkg/push"

	canaryv1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/aggregate"
	"github.com/flanksource/canary-checker/pkg/api"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/controllers"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/logger"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var Operator = &cobra.Command{
	Use:   "operator",
	Short: "Start the kubernetes operator",
	Run:   run,
}
var enableLeaderElection, dev bool
var httpPort, metricsPort, webhookPort int
var includeNamespace, includeCheck, prometheusURL string
var pushServers []string

func init() {
	Operator.Flags().IntVar(&httpPort, "httpPort", 8080, "Port to expose a health dashboard ")
	Operator.Flags().Int("devGuiHttpPort", 3004, "Port used by a local npm server in development mode")
	Operator.Flags().IntVar(&metricsPort, "metricsPort", 8081, "Port to expose a health dashboard ")
	Operator.Flags().IntVar(&webhookPort, "webhookPort", 8082, "Port for webhooks ")
	Operator.Flags().BoolVar(&dev, "dev", false, "Run in development mode")
	Operator.Flags().StringVar(&includeNamespace, "include-namespace", "", "Watch only specified namespaces, otherwise watch all")
	Operator.Flags().StringVar(&includeCheck, "include-check", "", "Run matching canaries - useful for debugging")

	Operator.Flags().BoolVar(&enableLeaderElection, "enable-leader-election", false, "Enabling this will ensure there is only one active controller manager")
	Operator.Flags().IntVar(&cache.Size, "maxStatusCheckCount", 5, "Maximum number of past checks in the status page")
	Operator.Flags().StringSliceVar(&aggregate.Servers, "aggregateServers", []string{}, "Aggregate check results from multiple servers in the status page")
	Operator.Flags().StringSliceVar(&pushServers, "push-servers", []string{}, "push check results to multiple canary servers")
	Operator.Flags().StringVar(&api.RunnerName, "name", "local", "Server name shown in aggregate dashboard")
	Operator.Flags().BoolVar(&aggregate.PivotByNamespace, "pivot-by-namespace", false, "Show the same check across namespaces in a different column")
	Operator.Flags().StringVar(&prometheusURL, "prometheus-url", "", "location of the prometheus server")
	// +kubebuilder:scaffold:scheme
}

func run(cmd *cobra.Command, args []string) {
	zapLogger := logger.GetZapLogger()
	if zapLogger == nil {
		logger.Fatalf("failed to get zap logger")
		return
	}

	loggr := ctrlzap.NewRaw(
		ctrlzap.UseDevMode(true),
		ctrlzap.WriteTo(os.Stderr),
		ctrlzap.Level(zapLogger.Level),
		ctrlzap.StacktraceLevel(zapLogger.StackTraceLevel),
		ctrlzap.Encoder(zapLogger.GetEncoder()),
	)

	scheme := runtime.NewScheme()

	_ = clientgoscheme.AddToScheme(scheme)
	_ = canaryv1.AddToScheme(scheme)
	go serve(cmd)

	ctrl.SetLogger(zapr.NewLogger(loggr))
	setupLog := ctrl.Log.WithName("setup")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: fmt.Sprintf("0.0.0.0:%d", metricsPort),
		Port:               webhookPort,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "bc88107d.flanksource.com",
	})

	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if api.RunnerName == "" {
		api.RunnerName = pkg.GetClusterName(mgr.GetConfig())
	}
	loggr.Sugar().Infof("Using runner name: %s", api.RunnerName)

	includeNamespaces := []string{}
	if includeNamespace != "" {
		includeNamespaces = strings.Split(includeNamespace, ",")
	}

	pkg.RunnerLabels = pkg.LoadLabels("/etc/podinfo/labels")

	reconciler := &controllers.CanaryReconciler{
		IncludeCheck:      includeCheck,
		IncludeNamespaces: includeNamespaces,
		Client:            mgr.GetClient(),
		Log:               ctrl.Log.WithName("controllers").WithName("canary"),
		Scheme:            mgr.GetScheme(),
	}

	if err = reconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Canary")
		os.Exit(1)
	}
	push.AddServers(pushServers)
	go push.Start()

	api.Prometheus, err = utils.NewPrometheusAPI(prometheusURL)
	if err != nil {
		logger.Debugf("error getting prometheus client")
		return
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
