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
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/obeleh/db-operator/shared"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=users,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=users/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=users/finalizers,verbs=update

type UserReco struct {
	Reco
	user  dboperatorv1alpha1.User
	users map[string]shared.DbSideUser
	conn  shared.DbServerConnectionInterface
}

func (r *UserReco) MarkedToBeDeleted() bool {
	return r.user.GetDeletionTimestamp() != nil
}

func (r *UserReco) LoadObj() (bool, error) {
	var err error
	dbServer, err := r.GetDbServer(r.user.Spec.DbServerName)
	if err != nil {
		if !shared.CannotFindError(err, r.Log, r.nsNm.Namespace, r.nsNm.Name) {
			r.LogError(err, "failed getting DbServer")
		}
		return false, err
	}
	r.conn, err = r.GetDbConnection(dbServer, nil)
	if err != nil {
		errStr := err.Error()
		if !strings.Contains(errStr, "failed getting password failed to get secret") {
			r.LogError(err, "failed getting dbInfo")
		}
		return false, err
	}

	r.users, err = r.conn.GetUsers()
	if err != nil {
		return false, err
	}
	_, exists := r.users[r.user.Spec.UserName]
	return exists, nil
}

func (r *UserReco) CreateObj() (ctrl.Result, error) {
	password, err := GetUserPassword(&r.user, r.client, r.ctx)
	if err != nil {
		r.LogError(err, fmt.Sprint(err))
		return ctrl.Result{
			// Gradual backoff
			Requeue:      true,
			RequeueAfter: time.Duration(time.Since(r.user.GetCreationTimestamp().Time).Seconds()),
		}, nil
	}
	r.Log.Info(fmt.Sprintf("Creating user %s", r.user.Spec.UserName))
	err = r.conn.CreateUser(r.user.Spec.UserName, *password)
	if err != nil {
		r.LogError(err, fmt.Sprintf("Failed to create user %s", r.user.Spec.UserName))
		return ctrl.Result{
			// Gradual backoff
			Requeue:      true,
			RequeueAfter: time.Duration(time.Since(r.user.GetCreationTimestamp().Time).Seconds()),
		}, err
	}

	_, err = r.EnsureCorrect()
	if err != nil {
		r.LogError(err, fmt.Sprint(err))
		return ctrl.Result{
			// Gradual backoff
			Requeue:      true,
			RequeueAfter: time.Duration(time.Since(r.user.GetCreationTimestamp().Time).Seconds()),
		}, err
	}
	return ctrl.Result{}, nil
}

func (r *UserReco) RemoveObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Dropping user %s", r.user.Spec.UserName))
	err := r.conn.DropUser(r.user.Spec.UserName)
	if err != nil {
		r.LogError(err, fmt.Sprintf("Failed to drop user %s", r.user.Spec.UserName))
		return ctrl.Result{
			// Gradual backoff
			Requeue:      true,
			RequeueAfter: time.Duration(time.Since(r.user.GetDeletionTimestamp().Time).Seconds()),
		}, err
	}
	r.Log.Info(fmt.Sprintf("finalized user %s", r.user.Spec.UserName))
	return ctrl.Result{}, nil
}

func (r *UserReco) LoadCR() (ctrl.Result, error) {
	err := r.client.Get(r.ctx, r.nsNm, &r.user)
	if err != nil {
		r.Log.Info(fmt.Sprintf("%T: %s does not exist", r.user, r.nsNm.Name))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *UserReco) GetCR() client.Object {
	return &r.user
}

func (r *UserReco) EnsureCorrect() (bool, error) {
	errors := []error{}
	resolvedDbNamePrivs := []dboperatorv1alpha1.DbPriv{}
	for _, dbPriv := range r.user.Spec.DbPrivs {
		db := dboperatorv1alpha1.Db{}
		nsm := types.NamespacedName{
			Name:      dbPriv.DbName,
			Namespace: r.user.Namespace,
		}
		err := r.client.Get(r.ctx, nsm, &db)
		if err == nil {
			resolvedDbNamePrivs = append(resolvedDbNamePrivs, dboperatorv1alpha1.DbPriv{
				DbName: db.Spec.DbName,
				Privs:  dbPriv.Privs,
			})
		} else {
			errors = append(errors, err)
		}
	}
	changes, err := r.conn.UpdateUserPrivs(r.user.Spec.UserName, r.user.Spec.ServerPrivs, resolvedDbNamePrivs)
	if err != nil {
		r.LogError(err, "Failed updating user privs")
		errors = append(errors, err)
	}
	var errsErr error
	if len(errors) > 0 {
		errsErr = fmt.Errorf("Got errors makeing sure user has correct privileges %s", errors)
	} else {
		errsErr = nil
	}
	return changes, errsErr
}

func (r *UserReco) CleanupConn() {
	if r.conn != nil {
		r.conn.Close()
	}
}

func (r *UserReco) NotifyChanges() {
	r.Log.Info("Notifying of User changes")
	reconcileRequest := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      r.user.Spec.DbServerName,
			Namespace: r.user.Namespace,
		},
	}

	reco := DbServerReconciler{
		r.client,
		r.Log,
		r.client.Scheme(),
	}

	reco.Reconcile(context.TODO(), reconcileRequest)
}

func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("user", req.NamespacedName)
	ur := UserReco{
		Reco: Reco{
			r.Client, ctx, r.Log, req.NamespacedName,
		},
	}
	return ur.Reco.Reconcile(&ur)
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.User{}).
		Complete(r)
}

func GetUserPassword(dbUser *dboperatorv1alpha1.User, k8sClient client.Client, ctx context.Context) (*string, error) {
	secretName := types.NamespacedName{
		Name:      dbUser.Spec.SecretName,
		Namespace: dbUser.Namespace,
	}
	secret := &v1.Secret{}
	err := k8sClient.Get(ctx, secretName, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %s", dbUser.Spec.SecretName)
	}

	passBytes, ok := secret.Data[shared.Nvl(dbUser.Spec.SecretKey, "password")]
	if !ok {
		return nil, fmt.Errorf("password key (%s) not found in secret", shared.Nvl(dbUser.Spec.SecretKey, "password"))
	}

	password := string(passBytes)

	return &password, nil
}
