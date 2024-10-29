package cmd

import (
	"errors"
	"fmt"
	"time"

	apicontext "github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/jobs"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/go-logr/logr"
	gocache "github.com/patrickmn/go-cache"

	canaryv1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/controllers"
	"github.com/flanksource/canary-checker/pkg/labels"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/job"
	dutyKubernetes "github.com/flanksource/duty/kubernetes"
	"github.com/flanksource/duty/leader"
	"github.com/flanksource/duty/shutdown"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlCache "sigs.k8s.io/controller-runtime/pkg/cache"
	ctrlMetrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	webhookPort          int
	k8sLogLevel          int
	enableLeaderElection bool
	Operator             = &cobra.Command{
		Use:   "operator",
		Short: "Start the kubernetes operator",
		Run: func(cmd *cobra.Command, args []string) {
			if err := run(); err != nil {
				shutdown.ShutdownAndExit(1, err.Error())
			} else {
				shutdown.ShutdownAndExit(0, "")
			}
		},
	}
)

func init() {
	ServerFlags(Operator.Flags())
	Operator.Flags().StringVarP(&runner.WatchNamespace, "namespace", "n", "", "Watch only specified namespace, otherwise watch all")
	Operator.Flags().BoolVar(&runner.OperatorExecutor, "executor", true, "If false, only serve the UI and sync the configs")
	Operator.Flags().IntVar(&webhookPort, "webhookPort", 8082, "Port for webhooks ")
	Operator.Flags().IntVar(&k8sLogLevel, "k8s-log-level", -1, "Kubernetes controller log level")
	Operator.Flags().BoolVar(&enableLeaderElection, "enable-leader-election", false, "Enabling this will ensure there is only one active controller manager")
	// +kubebuilder:scaffold:scheme
}

func run() error {
	ctx, err := InitContext()
	if err != nil {
		return err
	}

	if ctx.DB() == nil {
		return errors.New("operator requires a db connection")
	}

	if ctx.KubernetesRestConfig() == nil {
		return errors.New("operator requires a kubernetes connection")
	}

	ctx.WithTracer(otel.GetTracerProvider().Tracer(app))

	apicontext.DefaultContext = ctx.WithNamespace(runner.WatchNamespace)

	cache.PostgresCache = cache.NewPostgresCache(apicontext.DefaultContext)

	if enableLeaderElection {
		job.DisableCronStartOnSchedule()

		go func() {
			err := leader.Register(ctx, app, runner.WatchNamespace, nil, nil, nil)
			if err != nil {
				shutdown.ShutdownAndExit(1, fmt.Sprintf("leader election failed: %v", err))
			}
		}()
	}

	if runner.OperatorExecutor {
		logger.Infof("Starting executors")

		// Some synchronous jobs can take time
		// so we use a goroutine to unblock server start
		// to prevent health check from failing
		go jobs.Start()
	}

	go serve()
	return startControllers()
}

func startControllers() error {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = canaryv1.AddToScheme(scheme)

	logger := logger.GetLogger("operator")
	logger.SetLogLevel(k8sLogLevel)

	ctrl.SetLogger(logr.FromSlogHandler(logger.Handler()))
	setupLog := ctrl.Log.WithName("setup")

	managerOpt := ctrl.Options{
		Scheme:                  scheme,
		LeaderElection:          enableLeaderElection,
		LeaderElectionNamespace: runner.WatchNamespace,
		LeaderElectionID:        "fa62cd4d.flanksource.com",
		Metrics: ctrlMetrics.Options{
			BindAddress: ":0",
		},
		Cache: ctrlCache.Options{
			SyncPeriod: utils.Ptr(1 * time.Hour),
		},
	}

	if runner.WatchNamespace != "" {
		if managerOpt.Cache.DefaultNamespaces == nil {
			managerOpt.Cache.DefaultNamespaces = make(map[string]ctrlCache.Config)
		}
		managerOpt.Cache.DefaultNamespaces[runner.WatchNamespace] = ctrlCache.Config{}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), managerOpt)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return fmt.Errorf("unable to start manager: %v", err)
	}

	if runner.RunnerName == "" {
		runner.RunnerName = dutyKubernetes.GetClusterName(mgr.GetConfig())
	}
	logger.Infof("Using runner name: %s", runner.RunnerName)

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

	if err = canaryReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Canary")
		return fmt.Errorf("unable to create canary controller: %v", err)
	}

	// Instantiate the canary status channel so the canary job can send updates on it.
	// We are adding a small buffer to prevent blocking
	canaryJobs.CanaryStatusChannel = make(chan canaryJobs.CanaryStatusPayload, 64)

	// Listen for status updates
	go canaryReconciler.Report()

	if err = systemReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "System")
		return fmt.Errorf("unable to create topology controller: %v", err)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return fmt.Errorf("problem running controller manager: %v", err)
	}

	return nil
}
