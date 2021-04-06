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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

type Reconcilable interface {
	CreateObj() (ctrl.Result, error)
	RemoveObj() (ctrl.Result, error)
	LoadCR() (ctrl.Result, error)
	LoadObj() (bool, error)
	GetCR() client.Object
	MarkedToBeDeleted() bool
}

type Reco struct {
	client client.Client
	ctx    context.Context
	Log    logr.Logger
	nsNm   types.NamespacedName
}

func (rc *Reco) Reconcile(rcl Reconcilable) (ctrl.Result, error) {
	res, err := rcl.LoadCR()
	if err != nil {
		// Not found
		return res, nil
	}

	cr := rcl.GetCR()
	markedToBeDeleted := cr.GetDeletionTimestamp() != nil

	exists, err := rcl.LoadObj()
	if !exists {
		res, err := rcl.CreateObj()
		if err == nil {
			if !controllerutil.ContainsFinalizer(cr, userFinalizer) {
				controllerutil.AddFinalizer(cr, userFinalizer)
				err = rc.client.Update(rc.ctx, cr)
				if err != nil {
					return ctrl.Result{}, err
				}
			}
		}
		return res, err
	} else {
		if markedToBeDeleted {
			if controllerutil.ContainsFinalizer(cr, userFinalizer) {
				res, err := rcl.RemoveObj()
				if err == nil {
					controllerutil.RemoveFinalizer(cr, userFinalizer)
					err = rc.client.Update(rc.ctx, cr)
					if err != nil {
						return ctrl.Result{}, err
					}
				}
				return res, err
			}
			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{}, nil
}

type UserConn struct {
	Reco
	user  dboperatorv1alpha1.User
	users map[string]PostgresUser
	conn  *sql.DB
}

func (uc *UserConn) MarkedToBeDeleted() bool {
	return uc.user.GetDeletionTimestamp() != nil
}

func (uc *UserConn) LoadObj() (bool, error) {
	var err error
	uc.conn, err = GetDbConnectionFromUser(uc.Log, uc.client, uc.ctx, &uc.user)
	if err != nil {
		return false, err
	}

	uc.users, err = GetUsers(uc.Log, uc.conn)
	if err != nil {
		return false, err
	}
	_, exists := uc.users[uc.user.Spec.UserName]
	return exists, nil
}

func (uc *UserConn) CreateObj() (ctrl.Result, error) {
	password, err := GetUserPassword(&uc.user, uc.client, uc.ctx)
	if err != nil {
		uc.Log.Error(err, fmt.Sprint(err))
		return ctrl.Result{Requeue: true}, nil
	}
	err = CreatePgUser(uc.user.Spec.UserName, *password, uc.conn)
	if err != nil {
		uc.Log.Error(err, fmt.Sprintf("Failed to create user %s", uc.user.Spec.UserName))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (uc *UserConn) RemoveObj() (ctrl.Result, error) {
	err := DropPgUser(uc.user.Spec.UserName, uc.conn)
	if err != nil {
		uc.Log.Error(err, fmt.Sprintf("Failed to drop user %s", uc.user.Spec.UserName))
		return ctrl.Result{}, err
	}
	uc.Log.Info(fmt.Sprintf("finalized user %s", uc.user.Spec.UserName))
	return ctrl.Result{}, nil
}

func (uc *UserConn) LoadCR() (ctrl.Result, error) {
	err := uc.client.Get(uc.ctx, uc.nsNm, &uc.user)
	if err != nil {
		uc.Log.Info(fmt.Sprintf("%T: %s does not exist", uc.user, uc.nsNm.Name))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (uc *UserConn) GetCR() client.Object {
	return &uc.user
}

const userFinalizer = "db-operator.kubemaster.com/finalizer"

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=users,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=users/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=users/finalizers,verbs=update

func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("user", req.NamespacedName)
	uc := UserConn{}
	uc.Reco = Reco{r.Client, ctx, r.Log, req.NamespacedName}
	return uc.Reco.Reconcile(&uc)
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.User{}).
		Complete(r)
}
