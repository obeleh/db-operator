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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
)

// DbReconciler reconciles a Db object
type DbReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

type DbReco struct {
	Reco
	db   dboperatorv1alpha1.Db
	dbs  map[string]PostgresDb
	conn *sql.DB
}

func (dr *DbReco) MarkedToBeDeleted() bool {
	return dr.db.GetDeletionTimestamp() != nil
}

func (dr *DbReco) LoadObj() (bool, error) {
	var err error
	dr.conn, err = GetDbConnectionFromDb(dr.client, dr.ctx, &dr.db)
	if err != nil {
		return false, err
	}

	dr.dbs, err = GetDbs(dr.conn)
	if err != nil {
		return false, err
	}
	_, exists := dr.dbs[dr.db.Spec.DbName]
	return exists, nil
}

func (dr *DbReco) CreateObj() (ctrl.Result, error) {
	userNsName := types.NamespacedName{
		Name:      dr.db.Spec.Owner,
		Namespace: dr.nsNm.Namespace,
	}
	dbUser := &dboperatorv1alpha1.User{}
	err := dr.client.Get(dr.ctx, userNsName, dbUser)
	if err != nil {
		dr.Log.Error(err, fmt.Sprintf("Failed to get User: %s", dr.db.Spec.Owner))
		return ctrl.Result{}, err
	}

	dr.Log.Info(fmt.Sprintf("Creating db %s", dr.db.Spec.DbName))
	err = CreateDb(dr.db.Spec.DbName, dbUser.Spec.UserName, dr.conn)
	if err != nil {
		dr.Log.Error(err, fmt.Sprintf("Failed to create Database: %s", dr.db.Spec.DbName))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (dr *DbReco) RemoveObj() (ctrl.Result, error) {
	if dr.db.Spec.DropOnDeletion {
		dr.Log.Info(fmt.Sprintf("Dropping db %s", dr.db.Spec.DbName))
		err := DropPgDb(dr.db.Spec.DbName, dr.conn)
		if err != nil {
			dr.Log.Error(err, fmt.Sprintf("Failed to drop db %s", dr.db.Spec.DbName))
			return ctrl.Result{}, err
		}
		dr.Log.Info(fmt.Sprintf("finalized db %s", dr.db.Spec.DbName))
	} else {
		dr.Log.Info(fmt.Sprintf("did not drop db %s as per spec", dr.db.Spec.DbName))
	}
	return ctrl.Result{}, nil
}

func (dr *DbReco) LoadCR() (ctrl.Result, error) {
	err := dr.client.Get(dr.ctx, dr.nsNm, &dr.db)
	if err != nil {
		dr.Log.Info(fmt.Sprintf("%T: %s does not exist", dr.db, dr.nsNm.Name))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (dr *DbReco) GetCR() client.Object {
	return &dr.db
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbs/finalizers,verbs=update
func (r *DbReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("db", req.NamespacedName)
	dr := DbReco{}
	dr.Reco = Reco{r.Client, ctx, r.Log, req.NamespacedName}
	return dr.Reco.Reconcile(&dr)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DbReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.Db{}).
		Complete(r)
}
