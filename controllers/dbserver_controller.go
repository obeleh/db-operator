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
	"net/http"
	"reflect"
	"sort"
	"time"

	_ "github.com/lib/pq"
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/shared"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create
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
		return shared.GradualBackoffRetry(dbServer.GetCreationTimestamp().Time), nil
	}

	reco := Reco{r.Client, ctx, r.Log, req.NamespacedName}

	deletionTimestamp := dbServer.GetDeletionTimestamp()
	markedToBeDeleted := deletionTimestamp != nil
	if markedToBeDeleted {
		destructionAgeInSecs := time.Since(deletionTimestamp.Time).Seconds()
		// for the first X secs, do nothing. Give other resources time to clean up
		if destructionAgeInSecs < 9 {
			return shared.GradualBackoffRetry(deletionTimestamp.Time), nil
		}
		err = reco.RemoveFinalizer(dbServer)
		if err != nil {
			r.LogError(err, "failed removing finalizer")
			return shared.RetryAfter(3), nil
		}
		return ctrl.Result{}, nil
	}

	databaseNames := []string{}
	userNames := []string{}
	var message string
	conn, err := reco.GetDbConnection(dbServer, nil, nil)
	if err != nil {
		r.SetStatus(dbServer, ctx, databaseNames, userNames, false, message)
		if !shared.IsHandledErr(err) {
			message = fmt.Sprintf("failed building dbConnection %s", err)
			r.LogError(err, message)
		}
		return shared.GradualBackoffRetry(dbServer.GetCreationTimestamp().Time), nil
	}

	defer conn.Close()
	databases, err := conn.GetDbs()
	if err != nil {
		message = fmt.Sprintf("Failed reading databases: %s", err)
		r.LogError(err, message)
		r.SetStatus(dbServer, ctx, databaseNames, userNames, false, message)
		return shared.RetryAfter(3), nil
	}
	for name := range databases {
		//r.Log.Info(fmt.Sprintf("Found DB %s", name))
		databaseNames = append(databaseNames, name)
	}

	users, err := conn.GetUsers()
	if err != nil {
		message = fmt.Sprintf("Failed reading users: %s", err)
		r.LogError(err, message)
		err = r.SetStatus(dbServer, ctx, databaseNames, userNames, false, message)
		if err != nil {
			return shared.RetryAfter(3), nil
		}
		return ctrl.Result{}, nil
	}
	for name := range users {
		userNames = append(userNames, name)
		//r.Log.Info(fmt.Sprintf("Found user %s", name))
	}

	err = r.SetStatus(dbServer, ctx, databaseNames, userNames, true, "successfully connected to database and retrieved users and databases")
	if err != nil {
		return shared.RetryAfter(3), nil
	}
	r.Log.Info("Done")
	_, err = reco.EnsureFinalizer(dbServer)
	if err != nil {
		return shared.RetryAfter(3), nil
	}
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

// Gets DB server, it will prefer the local namespace but can go through Global namespaces as well
func GetDbServer(dbServerName string, apiClient client.Client, localNamespace string) (*dboperatorv1alpha1.DbServer, error) {
	dbServerList := dboperatorv1alpha1.DbServerList{}

	err := apiClient.List(context.Background(), &dbServerList, &client.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, dbServer := range dbServerList.Items {
		if dbServer.Namespace == localNamespace {
			return &dbServer, nil
		}
	}

	cnt := len(dbServerList.Items)
	if cnt == 1 {
		dbServer := dbServerList.Items[0]
		return &dbServer, nil
	}

	if cnt == 0 {
		return nil, &errors.StatusError{ErrStatus: metav1.Status{
			Status: metav1.StatusFailure,
			Code:   http.StatusNotFound,
			Reason: metav1.StatusReasonNotFound,
		}}
	}

	// cnt > 1
	return nil, fmt.Errorf("Got %d results, unable to pick", cnt)
}
