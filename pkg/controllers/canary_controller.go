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
	"fmt"
	"time"

	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"
	"github.com/flanksource/canary-checker/pkg/runner"
	dutyContext "github.com/flanksource/duty/context"
	"github.com/go-logr/logr"
	jsontime "github.com/liamylian/jsontime/v2/v2"
	"github.com/nsf/jsondiff"
	"github.com/patrickmn/go-cache"
	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var json = jsontime.ConfigWithCustomTimeFormat

// CanaryReconciler reconciles a Canary object
type CanaryReconciler struct {
	LogPass, LogFail bool
	client.Client
	dutyContext.Context
	Log         logr.Logger
	Scheme      *runtime.Scheme
	Events      record.EventRecorder
	Cron        *cron.Cron
	RunnerName  string
	Done        chan *pkg.CheckResult
	CanaryCache *cache.Cache
}

const FinalizerName = "canary.canaries.flanksource.com"

// +kubebuilder:rbac:groups=canaries.flanksource.com,resources=canaries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=canaries.flanksource.com,resources=canaries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=*
// +kubebuilder:rbac:groups="",resources=pods/logs,verbs=*
func (r *CanaryReconciler) Reconcile(parentCtx gocontext.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("canary", req.NamespacedName)
	canary := &v1.Canary{}
	err := r.Get(parentCtx, req.NamespacedName, canary)
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{RequeueAfter: 10 * time.Minute}, corev1.ErrUnexpectedEndOfGroupGenerated
	}

	if runner.IsCanaryIgnored(&canary.ObjectMeta) {
		return ctrl.Result{}, nil
	}

	ctx := r.Context.WithObject(canary.ObjectMeta).WithName(req.NamespacedName.String())

	canary.SetRunnerName(r.RunnerName)

	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(canary, FinalizerName) {
		controllerutil.AddFinalizer(canary, FinalizerName)
		if err := r.Client.Update(ctx, canary); err != nil {
			logger.Error(err, "failed to update finalizers")
		}
	}

	if !canary.DeletionTimestamp.IsZero() {
		if err := db.DeleteCanary(ctx, canary.GetPersistedID()); err != nil {
			logger.Error(err, "failed to delete canary")
		}
		canaryJobs.Unschedule(canary.GetPersistedID())
		controllerutil.RemoveFinalizer(canary, FinalizerName)
		return ctrl.Result{}, r.Update(ctx, canary)
	}

	dbCanary, err := r.updateCanaryInDB(ctx, canary)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	// Sync jobs if canary is created or updated
	if canary.Generation == 1 {
		if err := canaryJobs.SyncCanaryJob(ctx, *dbCanary); err != nil {
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

	if val, ok := canary.Annotations["next-runtime"]; ok {
		runAt := utils.ParseTime(val)
		if runAt == nil {
			return ctrl.Result{}, fmt.Errorf("invalid next-runtime: %s", val)
		}

		if err := canaryJobs.TriggerAt(ctx, *dbCanary, *runAt); err != nil {
			return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Minute}, err
		}

		delete(canary.Annotations, "next-runtime")
		if err := r.Update(ctx, canary); err != nil {
			return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Minute}, err
		}
	}

	if canaryForStatus.Status.Replicas != canary.Spec.Replicas {
		if canary.Spec.Replicas == 0 {
			canaryJobs.Unschedule(canary.GetPersistedID())

			if err := r.suspendCanary(ctx, canary, true); err != nil {
				return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Minute}, err
			}
		} else {
			if err := r.suspendCanary(ctx, canary, false); err != nil {
				return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Minute}, err
			}

			if err := canaryJobs.SyncCanaryJob(ctx, *dbCanary); err != nil {
				return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Minute}, err
			}
		}

		canaryForStatus.Status.Replicas = canary.Spec.Replicas
	}

	canaryForStatus.Status.Checks = dbCanary.Checks
	canaryForStatus.Status.ObservedGeneration = canary.Generation
	if err = r.Status().Patch(ctx, &canaryForStatus, patch); err != nil {
		logger.Error(err, "failed to update status for canary")
		return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Minute}, err
	}

	return ctrl.Result{}, nil
}

func (r *CanaryReconciler) suspendCanary(ctx dutyContext.Context, canary *v1.Canary, suspend bool) error {
	if suspend {
		if canary.Annotations == nil {
			canary.Annotations = make(map[string]string)
		}
		canary.Annotations["suspend"] = "true"
	} else {
		delete(canary.Annotations, "suspend")
	}

	if err := r.Update(ctx, canary); err != nil {
		return err
	}

	return db.SuspendCanary(ctx, canary.GetPersistedID(), false)
}

func (r *CanaryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Events = mgr.GetEventRecorderFor("canary-checker")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Canary{}).
		Complete(r)
}

func (r *CanaryReconciler) persistAndCacheCanary(ctx dutyContext.Context, canary *v1.Canary) (*pkg.Canary, error) {
	dbCanary, changed, err := db.PersistCanary(ctx, *canary, "kubernetes/"+canary.GetPersistedID())
	if err != nil {
		return nil, err
	}
	r.CanaryCache.Set(dbCanary.ID.String(), dbCanary, cache.DefaultExpiration)

	if changed {
		if err := canaryJobs.SyncCanaryJob(ctx, *dbCanary); err != nil {
			return nil, err
		}
	}

	return dbCanary, nil
}

func (r *CanaryReconciler) updateCanaryInDB(ctx dutyContext.Context, canary *v1.Canary) (*pkg.Canary, error) {
	var dbCanary *pkg.Canary
	var err error

	// Get DBCanary from cache if exists else persist in database and update cache
	if cacheObj, exists := r.CanaryCache.Get(canary.GetPersistedID()); !exists {
		dbCanary, err = r.persistAndCacheCanary(ctx, canary)
		if err != nil {
			return nil, err
		}
	} else {
		dbCanary = cacheObj.(*pkg.Canary)
	}

	if changed, err := hasCanaryChanged(canary, dbCanary); err != nil {
		return nil, err
	} else if changed {
		dbCanary, err = r.persistAndCacheCanary(ctx, canary)
		if err != nil {
			return nil, err
		}
	}

	return dbCanary, nil
}

func hasCanaryChanged(canary *v1.Canary, dbCanary *pkg.Canary) (bool, error) {
	if !utils.IsMapIdentical(canary.Annotations, dbCanary.Annotations) {
		return true, nil
	}

	// Compare canary spec and spec in database
	// If they do not match, persist the canary in database
	canarySpecJSON, err := json.Marshal(canary.Spec)
	if err != nil {
		return false, err
	}

	opts := jsondiff.DefaultJSONOptions()
	diff, _ := jsondiff.Compare(canarySpecJSON, dbCanary.Spec, &opts)
	specChanged := diff != jsondiff.FullMatch
	return specChanged, nil
}

func (r *CanaryReconciler) Report() {
	for payload := range canaryJobs.CanaryStatusChannel {
		var canary v1.Canary
		err := r.Get(gocontext.Background(), payload.NamespacedName, &canary)
		if err != nil {
			r.Log.Error(err, "failed to get canary while reporting", "canary", payload.NamespacedName)
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
