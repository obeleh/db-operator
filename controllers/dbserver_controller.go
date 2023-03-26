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
	"reflect"
	"sort"
	"time"

	_ "github.com/lib/pq"
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/shared"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DbServerReconciler reconciles a DbServer object
type DbServerReconciler struct {
	client.Client
	Log    *zap.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbservers/finalizers,verbs=update

// Generic Kubebuilder rules:
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

func (r *DbServerReconciler) LogError(err error, message string) {
	r.Log.Error(fmt.Sprintf("%s Error: %s", message, err))
}

func (r *DbServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log = r.Log.With(zap.String("Namespace", req.Namespace)).With(zap.String("Name", req.Name))

	dbServer := &dboperatorv1alpha1.DbServer{}
	err := r.Get(ctx, req.NamespacedName, dbServer)
	if err != nil {
		if !shared.CannotFindError(err, r.Log, "DbServer", req.Namespace, req.Name) {
			r.LogError(err, fmt.Sprintf("Failed to get dbServer: %s", req.Name))
		}
		return ctrl.Result{}, nil
	}

	databaseNames := []string{}
	userNames := []string{}
	var message string

	reco := Reco{r.Client, ctx, r.Log, req.NamespacedName}
	conn, err := reco.GetDbConnection(dbServer, nil, nil)

	if err != nil {
		r.SetStatus(dbServer, ctx, databaseNames, userNames, false, message)
		if !shared.IsHandledErr(err) {
			message = fmt.Sprintf("failed building dbConnection %s", err)
			r.LogError(err, message)
			return shared.GradualBackoffRetry(dbServer.GetCreationTimestamp().Time), nil
		}
		return ctrl.Result{}, nil
	}

	defer conn.Close()
	databases, err := conn.GetDbs()
	if err != nil {
		message = fmt.Sprintf("Failed reading databases: %s", err)
		r.LogError(err, message)
		err = r.SetStatus(dbServer, ctx, databaseNames, userNames, false, message)
		if err != nil {
			return ctrl.Result{Requeue: true, RequeueAfter: time.Second}, nil
		}
		return ctrl.Result{}, nil
	}
	for name := range databases {
		r.Log.Info(fmt.Sprintf("Found DB %s", name))
		databaseNames = append(databaseNames, name)
	}

	users, err := conn.GetUsers()
	if err != nil {
		message = fmt.Sprintf("Failed reading users: %s", err)
		r.LogError(err, message)
		err = r.SetStatus(dbServer, ctx, databaseNames, userNames, false, message)
		if err != nil {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, nil
	}
	for name := range users {
		userNames = append(userNames, name)
		r.Log.Info(fmt.Sprintf("Found user %s", name))
	}

	err = r.SetStatus(dbServer, ctx, databaseNames, userNames, true, "successfully connected to database and retrieved users and databases")
	if err != nil {
		return ctrl.Result{Requeue: true}, nil
	}
	r.Log.Info("Done")
	return ctrl.Result{}, nil
}

func (r *DbServerReconciler) SetStatus(dbServer *dboperatorv1alpha1.DbServer, ctx context.Context, databaseNames []string, userNames []string, connectionAvailable bool, statusMessage string) error {
	sort.Strings(databaseNames)
	sort.Strings(userNames)

	newStatus := dboperatorv1alpha1.DbServerStatus{Databases: databaseNames, Users: userNames, ConnectionAvailable: connectionAvailable, Message: statusMessage}
	if !reflect.DeepEqual(dbServer.Status, newStatus) {
		dbServer.Status = newStatus
		err := r.Status().Update(ctx, dbServer)
		if err != nil {
			message := fmt.Sprintf("failed patching status %s", err)
			r.Log.Info(message)
			return fmt.Errorf(message)
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DbServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.DbServer{}).
		Complete(r)
}
