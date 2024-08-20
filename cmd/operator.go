package cmd

import (
	"os"
	"time"

	apicontext "github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/jobs"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/canary-checker/pkg/utils"
	gocache "github.com/patrickmn/go-cache"
	"go.uber.org/zap/zapcore"

	canaryv1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/controllers"
	"github.com/flanksource/canary-checker/pkg/labels"
	"github.com/flanksource/commons/logger"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlCache "sigs.k8s.io/controller-runtime/pkg/cache"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	ctrlMetrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var webhookPort int
var k8sLogLevel int
var enableLeaderElection bool
var operatorExecutor bool
var disablePostgrest bool
var Operator = &cobra.Command{
	Use:   "operator",
	Short: "Start the kubernetes operator",
	Run:   run,
}

func init() {
	ServerFlags(Operator.Flags())
	Operator.Flags().StringVarP(&runner.WatchNamespace, "namespace", "n", "", "Watch only specified namespace, otherwise watch all")
	Operator.Flags().BoolVar(&operatorExecutor, "executor", true, "If false, only serve the UI and sync the configs")
	Operator.Flags().IntVar(&webhookPort, "webhookPort", 8082, "Port for webhooks ")
	Operator.Flags().IntVar(&k8sLogLevel, "k8s-log-level", -1, "Kubernetes controller log level")
	Operator.Flags().BoolVar(&enableLeaderElection, "enable-leader-election", false, "Enabling this will ensure there is only one active controller manager")
	Operator.Flags().BoolVar(&disablePostgrest, "disable-postgrest", false, "Disable the postgrest server")
	// +kubebuilder:scaffold:scheme
}

func run(cmd *cobra.Command, args []string) {
	defer runner.Shutdown()
	loggr := ctrlzap.NewRaw(
		ctrlzap.Encoder(logger.NewZapEncoder()),
		ctrlzap.Level(zapcore.Level(k8sLogLevel*-1)),
	).Named("operator")

	scheme := runtime.NewScheme()

	_ = clientgoscheme.AddToScheme(scheme)
	_ = canaryv1.AddToScheme(scheme)

	ctx, err := InitContext()
	if err != nil {
		runner.ShutdownAndExit(1, err.Error())
	}

	if ctx.DB() == nil {
		runner.ShutdownAndExit(1, "operator requires a db connection")
	}

	if ctx.Kommons() == nil {
		runner.ShutdownAndExit(1, "operator requires a kubernetes connection")
	}

	ctx.WithTracer(otel.GetTracerProvider().Tracer("canary-checker"))

	apicontext.DefaultContext = ctx.WithNamespace(runner.WatchNamespace)

	cache.PostgresCache = cache.NewPostgresCache(apicontext.DefaultContext)

	if operatorExecutor {
		logger.Infof("Starting executors")

		// Some synchronous jobs can take time
		// so we use a goroutine to unblock server start
		// to prevent health check from failing
		go jobs.Start()
	}
	go serve()

	ctrl.SetLogger(zapr.NewLogger(loggr))
	setupLog := ctrl.Log.WithName("setup")

	managerOpt := ctrl.Options{
		Scheme:                  scheme,
		LeaderElection:          enableLeaderElection,
		LeaderElectionNamespace: runner.WatchNamespace,
		Metrics: ctrlMetrics.Options{
			BindAddress: ":0",
		},
		Cache: ctrlCache.Options{
			SyncPeriod: utils.Ptr(1 * time.Hour),
		},
	}

	if runner.WatchNamespace != "" {
		managerOpt.Cache.DefaultNamespaces[runner.WatchNamespace] = ctrlCache.Config{}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), managerOpt)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if runner.RunnerName == "" {
		runner.RunnerName = pkg.GetClusterName(mgr.GetConfig())
	}
	loggr.Sugar().Infof("Using runner name: %s", runner.RunnerName)

	runner.RunnerLabels = labels.LoadFromFile("/etc/podinfo/labels")

	canaryReconciler := &controllers.CanaryReconciler{
		Context:     apicontext.DefaultContext,
		Client:      mgr.GetClient(),
		LogPass:     logPass,
		LogFail:     logFail,
		Log:         ctrl.Log.WithName("controllers").WithName("canary"),
		Scheme:      mgr.GetScheme(),
		RunnerName:  runner.RunnerName,
		CanaryCache: gocache.New(7*24*time.Hour, 1*time.Hour),
	}

	systemReconciler := &controllers.TopologyReconciler{
		Context: apicontext.DefaultContext,
		Client:  mgr.GetClient(),
		Log:     ctrl.Log.WithName("controllers").WithName("system"),
		Scheme:  mgr.GetScheme(),
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
