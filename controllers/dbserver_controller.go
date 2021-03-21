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
	"database/sql"
	"fmt"

	"github.com/go-logr/logr"
	_ "github.com/lib/pq"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DbServer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *DbServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("dbserver", req.NamespacedName)

	dbServer := &dboperatorv1alpha1.DbServer{}
	err := r.Get(ctx, req.NamespacedName, dbServer)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("Failed to get dbServer: %s", req.Name))
		return ctrl.Result{}, nil
	}

	secretName := types.NamespacedName{
		Name:      dbServer.Spec.SecretName,
		Namespace: req.Namespace,
	}
	secret := &v1.Secret{}

	err = r.Get(ctx, secretName, secret)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("Failed to get secret: %s", dbServer.Spec.SecretName))
		return ctrl.Result{}, err
	}

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbServer.Spec.Address, dbServer.Spec.Port, dbServer.Spec.UserName, secret.Data[dbServer.Spec.SecretKey], "postgres")
	pgDbServer, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("Failed to open a DB connection: %s", psqlInfo))
	}
	defer pgDbServer.Close()

	rows, rerr := pgDbServer.Query("SELECT datname FROM pg_database WHERE datistemplate = false;")
	if rerr != nil {
		r.Log.Error(rerr, fmt.Sprintf("Unable to read databases from server: %s", dbServer.Name))
	}
	for rows.Next() {
		var databaseName string
		err = rows.Scan(&databaseName)
		if err != nil {
			break
		}
		r.Log.Info(fmt.Sprintf("Found datbase: %s", databaseName))
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
