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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

type UserReco struct {
	Reco
	user  dboperatorv1alpha1.User
	users map[string]PostgresUser
	conn  *sql.DB
}

func (ur *UserReco) MarkedToBeDeleted() bool {
	return ur.user.GetDeletionTimestamp() != nil
}

func (ur *UserReco) LoadObj() (bool, error) {
	var err error
	ur.conn, err = GetDbConnectionFromUser(ur.client, ur.ctx, &ur.user)
	if err != nil {
		return false, err
	}

	ur.users, err = GetUsers(ur.conn)
	if err != nil {
		return false, err
	}
	_, exists := ur.users[ur.user.Spec.UserName]
	return exists, nil
}

func (ur *UserReco) CreateObj() (ctrl.Result, error) {
	password, err := GetUserPassword(&ur.user, ur.client, ur.ctx)
	if err != nil {
		ur.Log.Error(err, fmt.Sprint(err))
		return ctrl.Result{Requeue: true}, nil
	}
	err = CreatePgUser(ur.user.Spec.UserName, *password, ur.conn)
	if err != nil {
		ur.Log.Error(err, fmt.Sprintf("Failed to create user %s", ur.user.Spec.UserName))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (ur *UserReco) RemoveObj() (ctrl.Result, error) {
	err := DropPgUser(ur.user.Spec.UserName, ur.conn)
	if err != nil {
		ur.Log.Error(err, fmt.Sprintf("Failed to drop user %s", ur.user.Spec.UserName))
		return ctrl.Result{}, err
	}
	ur.Log.Info(fmt.Sprintf("finalized user %s", ur.user.Spec.UserName))
	return ctrl.Result{}, nil
}

func (ur *UserReco) LoadCR() (ctrl.Result, error) {
	err := ur.client.Get(ur.ctx, ur.nsNm, &ur.user)
	if err != nil {
		ur.Log.Info(fmt.Sprintf("%T: %s does not exist", ur.user, ur.nsNm.Name))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (ur *UserReco) GetCR() client.Object {
	return &ur.user
}

const userFinalizer = "db-operator.kubemaster.com/finalizer"

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=users,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=users/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=users/finalizers,verbs=update

func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("user", req.NamespacedName)
	ur := UserReco{}
	ur.Reco = Reco{r.Client, ctx, r.Log, req.NamespacedName}
	return ur.Reco.Reconcile(&ur)
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.User{}).
		Complete(r)
}
