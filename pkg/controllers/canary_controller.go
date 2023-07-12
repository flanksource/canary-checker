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
	gocontext "context"
	"time"

	"github.com/flanksource/canary-checker/pkg/db"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"
	"github.com/flanksource/kommons"
	"github.com/go-logr/logr"
	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
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
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	canary.SetRunnerName(r.RunnerName)
	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(canary, FinalizerName) {
		controllerutil.AddFinalizer(canary, FinalizerName)
		if err := r.Client.Update(ctx, canary); err != nil {
			logger.Error(err, "failed to update finalizers")
		}
	}

	if !canary.DeletionTimestamp.IsZero() {
		if err := db.DeleteCanary(*canary); err != nil {
			logger.Error(err, "failed to delete canary")
		}
		canaryJobs.DeleteCanaryJob(canary.GetPersistedID())
		controllerutil.RemoveFinalizer(canary, FinalizerName)
		return ctrl.Result{}, r.Update(ctx, canary)
	}

	_, checks, changed, err := db.PersistCanary(*canary, "kubernetes/"+string(canary.ObjectMeta.UID))
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	var checkIDs []string
	for _, id := range checks {
		checkIDs = append(checkIDs, id)
	}

	// Sync jobs if canary is created or updated
	if canary.Generation == 1 || changed {
		if err := canaryJobs.SyncCanaryJob(*canary); err != nil {
			logger.Error(err, "failed to sync canary job")
			return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Minute}, err
		}
	}

	// Update status
	var canaryForStatus v1.Canary
	err = r.Get(ctx, req.NamespacedName, &canaryForStatus)
	if err != nil {
		logger.Error(err, "Error fetching canary for status update")
		return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Minute}, err
	}
	patch := client.MergeFrom(canaryForStatus.DeepCopy())

	canaryForStatus.Status.Checks = checks
	canaryForStatus.Status.ObservedGeneration = canary.Generation
	if err = r.Status().Patch(ctx, &canaryForStatus, patch); err != nil {
		logger.Error(err, "failed to update status for canary")
		return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Minute}, err
	}
	return ctrl.Result{}, nil
}

func (r *CanaryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Events = mgr.GetEventRecorderFor("canary-checker")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Canary{}).
		Complete(r)
}

func (r *CanaryReconciler) Report() {
	for payload := range canaryJobs.CanaryStatusChannel {
		var canary v1.Canary
		err := r.Get(gocontext.Background(), payload.NamespacedName, &canary)
		if err != nil {
			r.Log.Error(err, "failed to get canary", "canary", canary.Name)
			continue
		}

		patch := client.MergeFrom(canary.DeepCopy())
		canary.Status.Latency1H = payload.Latency
		canary.Status.Uptime1H = payload.Uptime
		if payload.LastTransitionedTime != nil {
			canary.Status.LastTransitionedTime = payload.LastTransitionedTime
		}

		canary.Status.Message = &payload.Message
		canary.Status.ErrorMessage = &payload.ErrorMessage

		canary.Status.LastCheck = &metav1.Time{Time: time.Now()}
		canary.Status.ChecksStatus = payload.CheckStatus
		if payload.Pass {
			canary.Status.Status = &v1.Passed
		} else {
			canary.Status.Status = &v1.Failed
		}

		for _, eventMsg := range payload.FailEvents {
			r.Events.Event(&canary, corev1.EventTypeWarning, "Failed", eventMsg)
		}

		if err := r.Status().Patch(gocontext.Background(), &canary, patch); err != nil {
			r.Log.Error(err, "failed to update status", "canary", canary.Name)
		}
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
