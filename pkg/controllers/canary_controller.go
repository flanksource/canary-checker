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
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/flanksource/kommons"

	"github.com/flanksource/commons/logger"
	"github.com/flanksource/kommons/ktemplate"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/go-logr/logr"
	"github.com/mitchellh/reflectwalk"
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
	client.Client
	Kubernetes kubernetes.Interface
	Kommons    *kommons.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	Events     record.EventRecorder
	Cron       *cron.Cron
	Done       chan *pkg.CheckResult
}

// track the canaries that have already been scheduled
var observed = sync.Map{}

const FinalizerName = "canary.canaries.flanksource.com"

// +kubebuilder:rbac:groups=canaries.flanksource.com,resources=canaries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=canaries.flanksource.com,resources=canaries/status,verbs=get;update;patch
func (r *CanaryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if len(r.IncludeNamespaces) > 0 && !r.includeNamespace(req.Namespace) {
		r.Log.V(2).Info("namespace not included, skipping")
		return ctrl.Result{}, nil
	}
	if r.IncludeCheck != "" && r.IncludeCheck != req.Name {
		r.Log.V(2).Info("check not included, skipping")
		return ctrl.Result{}, nil
	}

	logger := r.Log.WithValues("canary", req.NamespacedName)

	check := &v1.Canary{}
	err := r.Get(ctx, req.NamespacedName, check)

	if !check.DeletionTimestamp.IsZero() {
		logger.Info("removing")
		cache.RemoveCheck(*check)
		metrics.RemoveCheck(*check)
		controllerutil.RemoveFinalizer(check, FinalizerName)
		if err := r.Update(ctx, check); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	} else if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	defer r.Patch(check)
	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(check, FinalizerName) {
		logger.Info("adding finalizer", "finalizers", check.GetFinalizers())
		controllerutil.AddFinalizer(check, FinalizerName)
		logger.Info("added finalizer", "finalizers", check.GetFinalizers())
		if err := r.Update(ctx, check); err != nil {
			return ctrl.Result{}, err
		}
	}

	_, run := observed.Load(req.NamespacedName)
	if run && check.Status.ObservedGeneration == check.Generation {
		logger.V(2).Info("check already up to date")
		return ctrl.Result{}, nil
	}

	observed.Store(req.NamespacedName, true)
	cache.Cache.InitCheck(*check)
	for _, entry := range r.Cron.Entries() {
		if entry.Job.(CanaryJob).GetNamespacedName() == req.NamespacedName {
			logger.V(2).Info("unscheduled", "id", entry.ID)
			r.Cron.Remove(entry.ID)
			break
		}
	}

	if check.Spec.Interval > 0 {
		job := CanaryJob{Client: *r, Check: *check, Logger: logger}
		if !run {
			// check each job on startup
			go job.Run()
		}
		id, err := r.Cron.AddJob(fmt.Sprintf("@every %ds", check.Spec.Interval), job)
		if err != nil {
			logger.Error(err, "failed to schedule job", "schedule", check.Spec.Interval)
		} else {
			logger.Info("scheduled", "id", id, "next", r.Cron.Entry(id).Next)
		}
	}

	check.Status.ObservedGeneration = check.Generation

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

func (r *CanaryReconciler) Report(key types.NamespacedName, results []*pkg.CheckResult) {
	check := v1.Canary{}
	if err := r.Get(context.TODO(), key, &check); err != nil {
		r.Log.Error(err, "unable to find canary", "key", key)
		return
	}

	check.Status.LastCheck = &metav1.Time{Time: time.Now()}
	transitioned := false
	pass := true
	for _, result := range results {
		lastResult := cache.AddCheck(check, result)
		//FIXME this needs to be aggregated across all
		uptime, latency := metrics.Record(check, result)
		check.Status.Uptime1H = uptime
		check.Status.Latency1H = latency.String()
		if lastResult != nil && len(lastResult.Statuses) > 0 && (lastResult.Statuses[0].Status != result.Pass) {
			transitioned = true
		}
		if !result.Pass {
			r.Events.Event(&check, corev1.EventTypeWarning, "Failed", fmt.Sprintf("%s-%s: %s", result.Check.GetType(), result.Check.GetEndpoint(), result.Message))
		}

		if transitioned {
			check.Status.LastTransitionedTime = &metav1.Time{Time: time.Now()}
		}
		pass = pass && result.Pass
	}
	if pass {
		check.Status.Status = &v1.Passed
	} else {
		check.Status.Status = &v1.Failed
	}
	r.Patch(&check)
}

func (r *CanaryReconciler) Patch(canary *v1.Canary) {
	r.Log.V(1).Info("patching", "canary", canary.Name, "namespace", canary.Namespace, "status", canary.Status.Status)
	if err := r.Status().Update(context.TODO(), canary, &client.UpdateOptions{}); err != nil {
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
	Client CanaryReconciler
	Check  v1.Canary
	logr.Logger
}

func (c CanaryJob) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: c.Check.Name, Namespace: c.Check.Namespace}
}

func (c CanaryJob) Run() {
	spec, err := c.LoadSecrets()
	if err != nil {
		c.Error(err, "Failed to load secrets")
		return
	}
	c.V(2).Info("Starting")

	spec.SetNamespaces(c.Check.Namespace)

	var results []*pkg.CheckResult
	for _, check := range checks.All {
		switch cs := check.(type) {
		case checks.SetsClient:
			cs.SetClient(c.Client.Kommons)
		}
		results = append(results, check.Run(spec)...)
	}

	c.Client.Report(c.GetNamespacedName(), results)

	c.V(3).Info("Ending")
}

func (c CanaryJob) LoadSecrets() (v1.CanarySpec, error) {
	var values = make(map[string]string)

	for key, source := range c.Check.Spec.Env {
		val, err := v1.GetEnvVarRefValue(c.Client.Kubernetes, c.Check.Namespace, &source, &c.Check)
		if err != nil {
			return c.Check.Spec, err
		}
		values[key] = val
	}

	var val = &c.Check.Spec
	k8sclient, err := pkg.NewK8sClient()
	if err != nil {
		logger.Warnf("Could not create k8s client for templating: %v", err)
	}
	err = reflectwalk.Walk(val, ktemplate.StructTemplater{
		Values:    values,
		Clientset: k8sclient,
	})
	return *val, err
}
