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
	"fmt"

	"github.com/go-logr/logr"
	_ "github.com/lib/pq"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
)

// DbServerReconciler reconciles a DbServer object
type DbServerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbservers/finalizers,verbs=update
//+kubebuilder:rbac:resources=secret,verbs=get;list;watch
func (r *DbServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("dbserver", req.NamespacedName)

	dbServer := &dboperatorv1alpha1.DbServer{}
	err := r.Get(ctx, req.NamespacedName, dbServer)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("Failed to get dbServer: %s", req.Name))
		return ctrl.Result{}, nil
	}

	reco := Reco{r.Client, ctx, r.Log, req.NamespacedName}

	conn, err := reco.GetDbConnection(dbServer, nil)
	if err != nil {
		r.Log.Error(err, "failed building dbConnection")
		return ctrl.Result{}, err
	}

	defer conn.Close()
	databases, err := conn.GetDbs()
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("Failed reading databases: %s", err))
		return ctrl.Result{}, nil
	}
	for name, db := range databases {
		r.Log.Info(fmt.Sprintf("Found DB %s with Owner %s", name, db.Owner))
	}

	r.Log.Info("Done")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DbServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.DbServer{}).
		Complete(r)
}
