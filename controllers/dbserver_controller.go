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
	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
	_ "github.com/lib/pq"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// Generic Kubebuilder rules:
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

func (r *DbServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("dbserver", req.NamespacedName)

	r.Log.Info("A")
	dbServer := &dboperatorv1alpha1.DbServer{}
	err := r.Get(ctx, req.NamespacedName, dbServer)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("Failed to get dbServer: %s", req.Name))
		return ctrl.Result{}, nil
	}

	databaseNames := []string{}
	userNames := []string{}
	var message string

	reco := Reco{r.Client, ctx, r.Log, req.NamespacedName}

	r.Log.Info("B")
	conn, err := reco.GetDbConnection(dbServer, nil)
	if err != nil {
		message = fmt.Sprintf("failed building dbConnection %s", err)
		r.Log.Error(err, message)
		r.SetStatus(dbServer, ctx, databaseNames, userNames, false, message)
		return ctrl.Result{}, err
	}

	defer conn.Close()
	r.Log.Info("C")
	databases, err := conn.GetDbs()
	if err != nil {
		message = fmt.Sprintf("Failed reading databases: %s", err)
		r.Log.Error(err, message)
		r.SetStatus(dbServer, ctx, databaseNames, userNames, false, message)
		return ctrl.Result{}, nil
	}
	r.Log.Info("D")
	for name, db := range databases {
		r.Log.Info(fmt.Sprintf("Found DB %s with Owner %s", name, db.Owner))
		databaseNames = append(databaseNames, name)
	}

	r.Log.Info("E")
	users, err := conn.GetUsers()
	if err != nil {
		message = fmt.Sprintf("Failed reading users: %s", err)
		r.Log.Error(err, message)
		r.SetStatus(dbServer, ctx, databaseNames, userNames, false, message)
		return ctrl.Result{}, nil
	}
	r.Log.Info("F")
	for name := range users {
		userNames = append(userNames, name)
		r.Log.Info(fmt.Sprintf("Found user %s", name))
	}

	r.Log.Info("G")
	r.SetStatus(dbServer, ctx, databaseNames, userNames, true, "successfully connected to database and retrieved users and databases")
	r.Log.Info("Done")

	return ctrl.Result{}, nil
}

func (r *DbServerReconciler) SetStatus(dbServer *dboperatorv1alpha1.DbServer, ctx context.Context, databaseNames []string, userNames []string, connectionAvailable bool, statusMessage string) {
	dbServer.Status = dboperatorv1alpha1.DbServerStatus{Databases: databaseNames, Users: userNames, ConnectionAvailable: connectionAvailable, Message: statusMessage}
	err := r.Status().Update(ctx, dbServer)
	if err != nil {
		r.Log.Error(err, "failed patching status %s", err)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DbServerReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.DbServer{}).
		Complete(r)
}
