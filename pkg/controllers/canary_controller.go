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

	"github.com/flanksource/canary-checker/pkg/db"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
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
		logger.Info("adding finalizer", "finalizers", canary.GetFinalizers())
		controllerutil.AddFinalizer(canary, FinalizerName)
	}

	if !canary.DeletionTimestamp.IsZero() {
		if err := db.DeleteCanary(*canary); err != nil {
			logger.Error(err, "failed to delete canary")
		}
		DeleteCanaryJob(*canary)
		controllerutil.RemoveFinalizer(canary, FinalizerName)
		return ctrl.Result{}, r.Update(ctx, canary)
	}

	id, changed, err := db.PersistCanary(*canary, "kubernetes/"+string(canary.ObjectMeta.UID))
	if err != nil {
		return ctrl.Result{
			Requeue: true,
		}, err
	}
	canary.Status.PersistedID = &id
	// Sync jobs if canary is created or updated
	if canary.Generation == 1 || changed {
		SyncCanaryJob(*canary)
	}
	canary.Status.ObservedGeneration = canary.Generation
	r.Patch(canary)
	return ctrl.Result{}, nil
}

func (r *CanaryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Events = mgr.GetEventRecorderFor("canary-checker")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Canary{}).
		Complete(r)
}

func (r *CanaryReconciler) Report(ctx *context.Context, canary v1.Canary, results []*pkg.CheckResult) {

	// canary.Status.LastCheck = &metav1.Time{Time: time.Now()}
	// transitioned := false
	// var messages, errors []string
	// var checkStatus = make(map[string]*v1.CheckStatus)
	// var duration int64
	// var pass = true
	// var passed int
	// for _, result := range results {
	// 	if r.LogPass && result.Pass || r.LogFail && !result.Pass {
	// 		r.Log.Info(result.String())
	// 	}
	// 	duration += result.Duration
	// 	cache.PostgresCache.Add(pkg.FromV1(canary, result.Check), pkg.FromResult(*result))
	// 	uptime, latency := metrics.Record(canary, result)
	// 	checkKey := canary.GetKey(result.Check)
	// 	checkStatus[checkKey] = &v1.CheckStatus{}
	// 	checkStatus[checkKey].Uptime1H = uptime.String()
	// 	checkStatus[checkKey].Latency1H = latency.String()
	// 	q := cache.QueryParams{Check: checkKey, StatusCount: 1}
	// 	if canary.Status.LastTransitionedTime != nil {
	// 		q.Start = canary.Status.LastTransitionedTime.Format(time.RFC3339)
	// 	}
	// 	lastStatus, err := cache.PostgresCache.Query(q)
	// 	if err != nil || len(lastStatus) == 0 || len(lastStatus[0].Statuses) == 0 {
	// 		transitioned = true
	// 	} else if len(lastStatus) > 0 && (lastStatus[0].Statuses[0].Status != result.Pass) {
	// 		transitioned = true
	// 	}
	// 	if !result.Pass {
	// 		r.Events.Event(&canary, corev1.EventTypeWarning, "Failed", fmt.Sprintf("%s-%s: %s", result.Check.GetType(), result.Check.GetEndpoint(), result.Message))
	// 	} else {
	// 		passed++
	// 	}
	// 	if transitioned {
	// 		checkStatus[checkKey].LastTransitionedTime = &metav1.Time{Time: time.Now()}
	// 		canary.Status.LastTransitionedTime = &metav1.Time{Time: time.Now()}
	// 	}

	// 	pass = pass && result.Pass
	// 	if result.Message != "" {
	// 		messages = append(messages, result.Message)
	// 	}
	// 	if result.Error != "" {
	// 		errors = append(errors, result.Error)
	// 	}
	// 	checkStatus[checkKey].Message = &result.Message
	// 	checkStatus[checkKey].ErrorMessage = &result.Error
	// 	push.Queue(pkg.FromV1(canary, result.Check), pkg.FromResult(*result))
	// }

	// uptime, latency := metrics.Record(canary, &pkg.CheckResult{
	// 	Check: v1.Check{
	// 		Type: "canary",
	// 	},
	// 	Pass:     pass,
	// 	Duration: duration,
	// })
	// canary.Status.Latency1H = utils.Age(time.Duration(latency.Rolling1H) * time.Millisecond)
	// canary.Status.Uptime1H = uptime.String()

	// msg := ""
	// errorMsg := ""
	// if len(messages) == 1 {
	// 	msg = messages[0]
	// } else if len(messages) > 1 {
	// 	msg = fmt.Sprintf("%s, (%d more)", messages[0], len(messages)-1)
	// }
	// if len(errors) == 1 {
	// 	errorMsg = errors[0]
	// } else if len(errors) > 1 {
	// 	errorMsg = fmt.Sprintf("%s, (%d more)", errors[0], len(errors)-1)
	// }

	// canary.Status.Message = &msg
	// canary.Status.ErrorMessage = &errorMsg

	// canary.Status.ChecksStatus = checkStatus
	// if pass {
	// 	canary.Status.Status = &v1.Passed
	// } else {
	// 	canary.Status.Status = &v1.Failed
	// }
	// r.Patch(ctx, &canary)
}

func (r *CanaryReconciler) Patch(canary *v1.Canary) {
	r.Log.V(3).Info("patching", "canary", canary.Name, "namespace", canary.Namespace, "status", canary.Status.Status)
	if err := r.Update(gocontext.Background(), canary, &client.UpdateOptions{}); err != nil {
		r.Log.Error(err, "failed to patch", "canary", canary.Name)
	}
	if err := r.Status().Update(gocontext.Background(), canary); err != nil {
		r.Log.Error(err, "failed to update status", "canary", canary.Name)
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
