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

// TopologyReconciler reconciles a Canary object
type TopologyReconciler struct {
	client.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	Events     record.EventRecorder
	RunnerName string
}

const TopologyFinalizerName = "topology.canaries.flanksource.com"

// +kubebuilder:rbac:groups=canaries.flanksource.com,resources=topologies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=canaries.flanksource.com,resources=topologies/status,verbs=get;update;patch
func (r *TopologyReconciler) Reconcile(ctx gocontext.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("topology", req.NamespacedName)
	topology := &v1.Topology{}
	err := r.Get(ctx, req.NamespacedName, topology)
	if errors.IsNotFound(err) {
		logger.V(1).Info("Topology not found")
		return ctrl.Result{}, nil
	}
	if !controllerutil.ContainsFinalizer(topology, TopologyFinalizerName) {
		controllerutil.AddFinalizer(topology, TopologyFinalizerName)
		if err := r.Client.Update(ctx, topology); err != nil {
			logger.Error(err, "failed to update finalizers")
		}
	}

	if !topology.DeletionTimestamp.IsZero() {
		if err := db.DeleteTopology(topology); err != nil {
			logger.Error(err, "failed to delete topology")
		}
		systemJobs.DeleteTopologyJob(*topology)
		controllerutil.RemoveFinalizer(topology, TopologyFinalizerName)
		return ctrl.Result{}, r.Update(ctx, topology)
	}

	id, changed, err := db.PersistTopology(topology)
	if err != nil {
		return ctrl.Result{}, err
	}

	// If template does not have a PersistedID, update its status to set one
	if topology.GetPersistedID() == "" {
		var topologyForStatus v1.Topology
		// We have to fetch the object again to avoid client caching old object
		err = r.Get(ctx, req.NamespacedName, &topologyForStatus)
		if err != nil {
			logger.Error(err, "Error fetching topology for status update")
			return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Minute}, err
		}

		topologyForStatus.Status.PersistedID = &id
		err = r.Status().Update(ctx, &topologyForStatus)
		if err != nil {
			logger.Error(err, "failed to update status for topology")
			return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Minute}, err
		}
	}

	// Sync jobs if system template is created or updated
	if changed || topology.Generation == 1 {
		if err := systemJobs.SyncTopologyJob(*topology); err != nil {
			logger.Error(err, "failed to sync topology job")
			return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Minute}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *TopologyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Events = mgr.GetEventRecorderFor("canary-checker")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Topology{}).
		Complete(r)
}
