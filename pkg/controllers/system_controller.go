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

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/db"
	systemJobs "github.com/flanksource/canary-checker/pkg/jobs/system"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// SystemReconciler reconciles a Canary object
type SystemReconciler struct {
	client.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	Events     record.EventRecorder
	RunnerName string
}

const SystemTemplateFinalizerName = "systemTemplate.canaries.flanksource.com"

// +kubebuilder:rbac:groups=canaries.flanksource.com,resources=systemtemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=canaries.flanksource.com,resources=systemtemplates/status,verbs=get;update;patch
func (r *SystemReconciler) Reconcile(ctx gocontext.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("system", req.NamespacedName)
	systemTemplate := &v1.SystemTemplate{}
	err := r.Get(ctx, req.NamespacedName, systemTemplate)
	if errors.IsNotFound(err) {
		logger.V(1).Info("System not found")
		return ctrl.Result{}, nil
	}
	if !controllerutil.ContainsFinalizer(systemTemplate, SystemTemplateFinalizerName) {
		logger.Info("adding finalizer", "finalizers", systemTemplate.GetFinalizers())
		controllerutil.AddFinalizer(systemTemplate, SystemTemplateFinalizerName)
	}

	if !systemTemplate.DeletionTimestamp.IsZero() {
		if err := db.DeleteSystemTemplate(systemTemplate); err != nil {
			logger.Error(err, "failed to delete system template")
		}
		systemJobs.DeleteSystemJob(*systemTemplate)
		controllerutil.RemoveFinalizer(systemTemplate, SystemTemplateFinalizerName)
		return ctrl.Result{}, r.Update(ctx, systemTemplate)
	}

	id, changed, err := db.PersistSystemTemplate(systemTemplate)
	if err != nil {
		return ctrl.Result{}, err
	}
	systemTemplate.Status.PersistedID = &id
	// Sync jobs if system template is created or updated
	if changed || systemTemplate.Status.ObservedGeneration == 1 {
		if err := systemJobs.SyncSystemJob(*systemTemplate); err != nil {
			logger.Error(err, "failed to sync system template job")
			return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Minute}, err
		}
	}
	systemTemplate.Status.ObservedGeneration = systemTemplate.Generation
	r.Patch(systemTemplate)
	return ctrl.Result{}, nil
}

func (r *SystemReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Events = mgr.GetEventRecorderFor("canary-checker")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.SystemTemplate{}).
		Complete(r)
}

func (r *SystemReconciler) Patch(systemTemplate *v1.SystemTemplate) {
	r.Log.V(3).Info("patching", "systemTemplate", systemTemplate.Name, "namespace", systemTemplate.Namespace, "status", systemTemplate.Status.Status)
	if err := r.Update(gocontext.Background(), systemTemplate, &client.UpdateOptions{}); err != nil {
		r.Log.Error(err, "failed to patch", "systemTemplate", systemTemplate.Name)
	}
	if err := r.Status().Update(gocontext.Background(), systemTemplate); err != nil {
		r.Log.Error(err, "failed to update status", "systemTemplate", systemTemplate.Name)
	}
}
