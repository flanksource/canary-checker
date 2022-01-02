/*
Copyright 2020 The Kubernetes authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"fmt"
	"sync"
	"time"

	gocontext "context"

	"github.com/flanksource/canary-checker/pkg/push"
	"github.com/flanksource/canary-checker/pkg/utils"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/kommons"
	"github.com/go-logr/logr"
	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// CanaryReconciler reconciles a Canary object
type CanaryReconciler struct {
	IncludeCheck      string
	IncludeNamespaces []string
	LogPass, LogFail  bool
	client.Client
	Kubernetes kubernetes.Interface
	Kommons    *kommons.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	Events     record.EventRecorder
	Cron       *cron.Cron
	RunnerName string
	Done       chan *pkg.CheckResult
}

// track the canaries that have already been scheduled
var observed = sync.Map{}

const FinalizerName = "canary.canaries.flanksource.com"

// +kubebuilder:rbac:groups=canaries.flanksource.com,resources=canaries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=canaries.flanksource.com,resources=canaries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=*
// +kubebuilder:rbac:groups="",resources=pods/logs,verbs=*
func (r *CanaryReconciler) Reconcile(ctx gocontext.Context, req ctrl.Request) (ctrl.Result, error) {
	if len(r.IncludeNamespaces) > 0 && !r.includeNamespace(req.Namespace) {
		r.Log.V(2).Info("namespace not included, skipping")
		return ctrl.Result{}, nil
	}
	if r.IncludeCheck != "" && r.IncludeCheck != req.Name {
		r.Log.V(2).Info("check not included, skipping")
		return ctrl.Result{}, nil
	}

	logger := r.Log.WithValues("canary", req.NamespacedName)

	canary := &v1.Canary{}
	err := r.Get(ctx, req.NamespacedName, canary)
	canary.SetRunnerName(r.RunnerName)
	var update bool
	if canary.Status.ChecksStatus != nil {
		specKeys := getAllCheckKeys(canary)
		for statusKey := range canary.Status.ChecksStatus {
			// TODO: figure out how the ResutMode generated check would be handled
			if !contains(specKeys, statusKey) && canary.Spec.ResultMode == "" {
				logger.Info("removing stale check", "key", statusKey)
				cache.CacheChain.RemoveCheckByKey(statusKey)
				metrics.RemoveCheckByKey(statusKey)
				update = true
			}
		}
	}
	if !canary.DeletionTimestamp.IsZero() {
		logger.Info("removing", "check", canary.Name)
		cache.CacheChain.RemoveChecks(*canary)
		metrics.RemoveCheck(*canary)
		controllerutil.RemoveFinalizer(canary, FinalizerName)
		if err := r.Update(ctx, canary); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	} else if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	canaryCtx := &context.Context{
		Kommons:   r.Kommons,
		Namespace: canary.GetNamespace(),
		Context:   ctx,
	}

	defer r.Patch(canaryCtx, canary)
	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(canary, FinalizerName) {
		logger.Info("adding finalizer", "finalizers", canary.GetFinalizers())
		controllerutil.AddFinalizer(canary, FinalizerName)
		if err := r.Update(ctx, canary); err != nil {
			return ctrl.Result{}, err
		}
	}
	canary.Spec.SetSQLDrivers()
	_, run := observed.Load(req.NamespacedName)
	if run && canary.Status.ObservedGeneration == canary.Generation && !update {
		logger.V(2).Info("check already up to date")
		return ctrl.Result{}, nil
	}

	observed.Store(req.NamespacedName, true)
	// since we are combining the checks and we don't want individual checks to be displayed on the UI.
	if canary.Spec.ResultMode == "" {
		cache.InMemoryCache.InitCheck(*canary)
	}
	// TODO shouldn't be deleting entries every time, only add once and remove and add new one if interval or schedule is changed.
	for _, entry := range r.Cron.Entries() {
		if entry.Job.(CanaryJob).GetNamespacedName() == req.NamespacedName {
			logger.V(2).Info("unscheduled", "id", entry.ID)
			r.Cron.Remove(entry.ID)
			break
		}
	}

	if canary.Spec.Interval > 0 || canary.Spec.Schedule != "" {
		job := CanaryJob{Client: *r, Canary: *canary, Logger: logger, Context: context.New(r.Kommons, *canary)}
		if !run {
			// check each job on startup
			go job.Run()
		}
		var id cron.EntryID
		var schedule string
		if canary.Spec.Schedule != "" {
			schedule = canary.Spec.Schedule
			id, err = r.Cron.AddJob(schedule, job)
		} else if canary.Spec.Interval > 0 {
			schedule = fmt.Sprintf("@every %ds", canary.Spec.Interval)
			id, err = r.Cron.AddJob(schedule, job)
		}
		if err != nil {
			logger.Error(err, "failed to schedule job", "schedule", schedule)
		} else {
			logger.V(2).Info("scheduled", "id", id, "next", r.Cron.Entry(id).Next)
		}
	}

	canary.Status.ObservedGeneration = canary.Generation
	return ctrl.Result{}, nil
}

func (r *CanaryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Events = mgr.GetEventRecorderFor("canary-checker")

	r.Cron = cron.New(cron.WithChain(
		cron.SkipIfStillRunning(r.Log),
	))
	r.Cron.Start()
	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}
	r.Kubernetes = clientset

	r.Kommons = kommons.NewClient(mgr.GetConfig(), logger.StandardLogger())
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Canary{}).
		Complete(r)
}

func (r *CanaryReconciler) Report(ctx *context.Context, canary v1.Canary, results []*pkg.CheckResult) {
	canary.Status.LastCheck = &metav1.Time{Time: time.Now()}
	transitioned := false
	var messages, errors []string
	var checkStatus = make(map[string]*v1.CheckStatus)
	var duration int64
	var pass = true
	var passed int
	for _, result := range results {
		if r.LogPass && result.Pass || r.LogFail && !result.Pass {
			r.Log.Info(result.String())
		}
		duration += result.Duration
		cache.CacheChain.Add(pkg.FromV1(canary, result.Check), pkg.FromResult(*result))
		uptime, latency := metrics.Record(canary, result)
		checkKey := canary.GetKey(result.Check)
		checkStatus[checkKey] = &v1.CheckStatus{}
		checkStatus[checkKey].Uptime1H = uptime.String()
		checkStatus[checkKey].Latency1H = latency.String()
		q := cache.QueryParams{Check: checkKey, StatusCount: 1}
		if canary.Status.LastTransitionedTime != nil {
			q.Start = canary.Status.LastTransitionedTime.Format(time.RFC3339)
		}
		lastStatus, err := cache.InMemoryCache.Query(q)
		if err != nil || len(lastStatus) == 0 || len(lastStatus[0].Statuses) == 0 {
			transitioned = true
		} else if len(lastStatus) > 0 && (lastStatus[0].Statuses[0].Status != result.Pass) {
			transitioned = true
		}
		if !result.Pass {
			r.Events.Event(&canary, corev1.EventTypeWarning, "Failed", fmt.Sprintf("%s-%s: %s", result.Check.GetType(), result.Check.GetEndpoint(), result.Message))
		} else {
			passed++
		}
		if transitioned {
			checkStatus[checkKey].LastTransitionedTime = &metav1.Time{Time: time.Now()}
			canary.Status.LastTransitionedTime = &metav1.Time{Time: time.Now()}
		}

		pass = pass && result.Pass
		if result.Message != "" {
			messages = append(messages, result.Message)
		}
		if result.Error != "" {
			errors = append(errors, result.Error)
		}
		checkStatus[checkKey].Message = &result.Message
		checkStatus[checkKey].ErrorMessage = &result.Error
		push.Queue(pkg.FromV1(canary, result.Check), pkg.FromResult(*result))
	}

	uptime, latency := metrics.Record(canary, &pkg.CheckResult{
		Check: v1.Check{
			Type: "canary",
		},
		Pass:     pass,
		Duration: duration,
	})
	canary.Status.Latency1H = utils.Age(time.Duration(latency.Rolling1H) * time.Millisecond)
	canary.Status.Uptime1H = uptime.String()

	msg := ""
	errorMsg := ""
	if len(messages) == 1 {
		msg = messages[0]
	} else if len(messages) > 1 {
		msg = fmt.Sprintf("%s, (%d more)", messages[0], len(messages)-1)
	}
	if len(errors) == 1 {
		errorMsg = errors[0]
	} else if len(errors) > 1 {
		errorMsg = fmt.Sprintf("%s, (%d more)", errors[0], len(errors)-1)
	}

	canary.Status.Message = &msg
	canary.Status.ErrorMessage = &errorMsg

	canary.Status.ChecksStatus = checkStatus
	if pass {
		canary.Status.Status = &v1.Passed
	} else {
		canary.Status.Status = &v1.Failed
	}
	r.Patch(ctx, &canary)
}

func (r *CanaryReconciler) Patch(ctx *context.Context, canary *v1.Canary) {
	r.Log.V(2).Info("patching", "canary", canary.Name, "namespace", canary.Namespace, "status", canary.Status.Status)
	if err := r.Status().Update(ctx, canary, &client.UpdateOptions{}); err != nil {
		r.Log.Error(err, "failed to patch", "canary", canary.Name)
	}
}

func (r *CanaryReconciler) includeNamespace(namespace string) bool {
	for _, n := range r.IncludeNamespaces {
		if n == namespace {
			return true
		}
	}
	return false
}

type CanaryJob struct {
	Client  CanaryReconciler
	Canary  v1.Canary
	Context *context.Context
	logr.Logger
}

func (c CanaryJob) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: c.Canary.Name, Namespace: c.Canary.Namespace}
}

func (c CanaryJob) Run() {
	c.V(2).Info("Starting")
	results := checks.RunChecks(c.Context)
	c.Client.Report(c.Context, c.Canary, results)

	c.V(3).Info("Ending")
}
