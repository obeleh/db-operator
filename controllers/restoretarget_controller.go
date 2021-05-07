/*
Copyright 2021.

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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
)

// RestoreTargetReconciler reconciles a RestoreTarget object
type RestoreTargetReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=restoretargets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=restoretargets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=restoretargets/finalizers,verbs=update

func (r *RestoreTargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("restoretarget", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.RestoreTarget{}).
		Complete(r)
}
