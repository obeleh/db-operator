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

	_ "github.com/lib/pq"
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/shared"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// https://yash-kukreja-98.medium.com/develop-on-kubernetes-series-demystifying-the-for-vs-owns-vs-watches-controller-builders-in-c11ab32a046e
var reconcileDbServerChannel chan event.GenericEvent

func InitializeDbServerChannel() {
	reconcileDbServerChannel = make(chan event.GenericEvent)
}

func TriggerDbServerReconcile(dbServer *dboperatorv1alpha1.DbServer) {
	reconcileDbServerChannel <- event.GenericEvent{Object: dbServer}
}

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
	r.Log.Info("Reconcile", zap.String("Namespace", req.Namespace), zap.String("Name", req.Name))
	dbServer := &dboperatorv1alpha1.DbServer{}
	err := r.Get(ctx, req.NamespacedName, dbServer)
	if err != nil {
		if !shared.CannotFindError(err, r.Log, "DbServer", req.Namespace, req.Name) {
			r.LogError(err, fmt.Sprintf("Failed to get dbServer: %s", req.Name))
		}
		return ctrl.Result{}, nil
	}

	reco := Reco{shared.K8sClient{Client: r.Client, Ctx: ctx, NsNm: req.NamespacedName, Log: r.Log}}

	deletionTimestamp := dbServer.GetDeletionTimestamp()
	markedToBeDeleted := deletionTimestamp != nil
	if markedToBeDeleted {
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
		r.SetStatus(dbServer, ctx, databaseNames, userNames, false, message, reco)
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
		r.SetStatus(dbServer, ctx, databaseNames, userNames, false, message, reco)
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
		err = r.SetStatus(dbServer, ctx, databaseNames, userNames, false, message, reco)
		if err != nil {
			return shared.RetryAfter(3), nil
		}
		return ctrl.Result{}, nil
	}
	for name := range users {
		userNames = append(userNames, name)
		//r.Log.Info(fmt.Sprintf("Found user %s", name))
	}

	err = r.SetStatus(dbServer, ctx, databaseNames, userNames, true, "successfully connected to database and retrieved users and databases", reco)
	if err != nil {
		return shared.RetryAfter(3), nil
	}
	r.Log.Info("Reconcile Done", zap.String("Namespace", req.Namespace), zap.String("Name", req.Name))
	return ctrl.Result{}, nil
}

func (r *DbServerReconciler) SetStatus(dbServer *dboperatorv1alpha1.DbServer, ctx context.Context, databaseNames []string, userNames []string, connectionAvailable bool, statusMessage string, reco Reco) error {
	sort.Strings(databaseNames)
	sort.Strings(userNames)
	changed := reco.AddFinalizerToCr(dbServer)
	newStatus := dboperatorv1alpha1.DbServerStatus{Databases: databaseNames, Users: userNames, ConnectionAvailable: connectionAvailable, Message: statusMessage}
	if changed {
		dbServer.Status = newStatus
		err := reco.Client.Update(reco.Ctx, dbServer)
		if err != nil {
			return err
		}
	} else if !reflect.DeepEqual(dbServer.Status, newStatus) {
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
	channelSource := source.Channel{Source: reconcileDbServerChannel}

	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.DbServer{}).
		WatchesRawSource(&channelSource, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

func (rc *DbServerReconciler) AddFinalizerToCr(cr client.Object) bool {
	if !controllerutil.ContainsFinalizer(cr, DB_OPERATOR_FINALIZER) {
		controllerutil.AddFinalizer(cr, DB_OPERATOR_FINALIZER)
		return true
	}
	return false
}

// Gets DB server, it will prefer the local namespace but can go through Global namespaces as well
func GetDbServer(dbServerName string, apiClient client.Client, localNamespace string) (*dboperatorv1alpha1.DbServer, error) {
	dbServerList := dboperatorv1alpha1.DbServerList{}

	err := apiClient.List(context.Background(), &dbServerList, &client.ListOptions{})
	if err != nil {
		return nil, err
	}

	var dbServers []dboperatorv1alpha1.DbServer
	for _, dbServer := range dbServerList.Items {
		if dbServer.Name == dbServerName {
			dbServers = append(dbServers, dbServer)
			if dbServer.Namespace == localNamespace {
				return &dbServer, nil
			}
		}
	}

	cnt := len(dbServers)
	if cnt == 1 {
		dbServer := dbServers[0]
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
	return nil, fmt.Errorf("got %d results, unable to pick", cnt)
}
