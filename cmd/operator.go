package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/jobs"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/canary-checker/pkg/utils"
	gocache "github.com/patrickmn/go-cache"

	canaryv1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/controllers"
	"github.com/flanksource/canary-checker/pkg/labels"
	"github.com/flanksource/commons/logger"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlCache "sigs.k8s.io/controller-runtime/pkg/cache"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var webhookPort int
var enableLeaderElection bool
var operatorNamespace string
var operatorExecutor bool
var disablePostgrest bool
var Operator = &cobra.Command{
	Use:   "operator",
	Short: "Start the kubernetes operator",
	Run:   run,
}

func init() {
	ServerFlags(Operator.Flags())
	Operator.Flags().StringVarP(&operatorNamespace, "namespace", "n", "", "Watch only specified namespaces, otherwise watch all")
	Operator.Flags().BoolVar(&operatorExecutor, "executor", true, "If false, only serve the UI and sync the configs")
	Operator.Flags().IntVar(&webhookPort, "webhookPort", 8082, "Port for webhooks ")
	Operator.Flags().BoolVar(&enableLeaderElection, "enable-leader-election", false, "Enabling this will ensure there is only one active controller manager")
	Operator.Flags().BoolVar(&disablePostgrest, "disable-postgrest", false, "Disable the postgrest server")
	// +kubebuilder:scaffold:scheme
}

func run(cmd *cobra.Command, args []string) {
	zapLogger := logger.GetZapLogger()
	if zapLogger == nil {
		logger.Fatalf("failed to get zap logger")
		return
	}
	canaryJobs.LogFail = logFail
	canaryJobs.LogPass = logPass

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

	if err := db.Init(); err != nil {
		logger.Fatalf("error connecting with postgres: %v", err)
	}
	cache.PostgresCache = cache.NewPostgresCache(db.Pool)
	if operatorExecutor {
		logger.Infof("Starting executors")
		jobs.Start()
	}
	go serve()

	ctrl.SetLogger(zapr.NewLogger(loggr))
	setupLog := ctrl.Log.WithName("setup")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      fmt.Sprintf("0.0.0.0:%d", metricsPort),
		Namespace:               operatorNamespace,
		Port:                    webhookPort,
		LeaderElection:          enableLeaderElection,
		LeaderElectionNamespace: operatorNamespace,
		LeaderElectionID:        "bc88107d.flanksource.com",
		Cache: ctrlCache.Options{
			SyncPeriod: utils.Ptr(1 * time.Hour),
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if runner.RunnerName == "" {
		runner.RunnerName = pkg.GetClusterName(mgr.GetConfig())
	}
	loggr.Sugar().Infof("Using runner name: %s", runner.RunnerName)

	includeNamespaces := []string{}
	if operatorNamespace != "" {
		includeNamespaces = strings.Split(operatorNamespace, ",")
		canaryJobs.CanaryNamespaces = includeNamespaces
	}
	runner.RunnerLabels = labels.LoadFromFile("/etc/podinfo/labels")

	canaryReconciler := &controllers.CanaryReconciler{
		IncludeCheck:      includeCheck,
		IncludeNamespaces: includeNamespaces,
		Client:            mgr.GetClient(),
		LogPass:           logPass,
		LogFail:           logFail,
		Log:               ctrl.Log.WithName("controllers").WithName("canary"),
		Scheme:            mgr.GetScheme(),
		RunnerName:        runner.RunnerName,
		CanaryCache:       gocache.New(7*24*time.Hour, 1*time.Hour),
	}

	systemReconciler := &controllers.TopologyReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("system"),
		Scheme: mgr.GetScheme(),
	}
	if err = mgr.Add(manager.RunnableFunc(db.Start)); err != nil {
		setupLog.Error(err, "unable to Add manager")
		os.Exit(1)
	}
	if err = canaryReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Canary")
		os.Exit(1)
	}

	// Instantiate the canary status channel so the canary job can send updates on it.
	// We are adding a small buffer to prevent blocking
	canaryJobs.CanaryStatusChannel = make(chan canaryJobs.CanaryStatusPayload, 64)

	// Listen for status updates
	go canaryReconciler.Report()

	if err = systemReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "System")
		os.Exit(1)
	}
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
	}
}
