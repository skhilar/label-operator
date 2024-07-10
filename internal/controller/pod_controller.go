/*
Copyright 2024.

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

package controller

import (
	"context"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	addTopologyLabelAnnotation  = "philips.com/add-topology-label"
	addTopologyRegion           = "topology.kubernetes.io/region"
	addTopologyAvailabilityZone = "topology.kubernetes.io/zone"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=nodes/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the Pod object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("pod", req.NamespacedName)
	var pod corev1.Pod

	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch Pod")
	}

	nodeName := pod.Spec.NodeName
	var node corev1.Node

	if err := r.Get(ctx, types.NamespacedName{Name: nodeName, Namespace: req.Namespace}, &node); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch node")
	}
	zoneLabel := node.Labels[addTopologyAvailabilityZone]
	regionLabel := node.Labels[addTopologyRegion]
	labelShouldBePresent := pod.Annotations[addTopologyLabelAnnotation] == "true"
	if labelShouldBePresent {
		labelIsPresent := pod.Labels[addTopologyAvailabilityZone] == zoneLabel
		if labelIsPresent {
			log.Info("no update required")
			return ctrl.Result{}, nil
		} else {
			if pod.Labels == nil {
				pod.Labels = make(map[string]string)
			}
			pod.Labels[addTopologyRegion] = regionLabel
			pod.Labels[addTopologyAvailabilityZone] = zoneLabel
			log.Info("adding label")
		}
	} else {
		if _, ok := pod.Labels[addTopologyRegion]; ok {
			delete(pod.Labels, addTopologyRegion)
		}
		if _, ok := pod.Labels[addTopologyAvailabilityZone]; ok {
			delete(pod.Labels, addTopologyAvailabilityZone)
		}
	}
	if err := r.Update(ctx, &pod); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "unable to update Pod")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}
