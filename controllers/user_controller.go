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
	"k8s.io/apimachinery/pkg/runtime"
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

const userFinalizer = "db-operator.kubemaster.com/finalizer"

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=users,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=users/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=users/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the User object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("user", req.NamespacedName)

	dbUser := &dboperatorv1alpha1.User{}
	err := r.Get(ctx, req.NamespacedName, dbUser)
	if err != nil {
		r.Log.Info(fmt.Sprintf("User: %s does not exist", req.Name))
		return ctrl.Result{}, nil
	}

	markedToBeDeleted := dbUser.GetDeletionTimestamp() != nil

	pgDbServer, err := GetDbConnectionFromUser(r.Log, r.Client, ctx, dbUser)
	if err != nil {
		return ctrl.Result{}, nil
	}
	defer pgDbServer.Close()

	users, err := GetUsers(r.Log, pgDbServer)
	if err != nil {
		return ctrl.Result{}, nil
	}
	_, exists := users[dbUser.Spec.UserName]

	if !exists {
		password, err := GetUserPassword(dbUser, r.Client, ctx)
		if err != nil {
			r.Log.Error(err, fmt.Sprint(err))
			return ctrl.Result{Requeue: true}, nil
		}
		err = CreatePgUser(dbUser.Spec.UserName, *password, pgDbServer)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("Failed to create user %s", dbUser.Spec.UserName))
			return ctrl.Result{}, err
		}

		if !controllerutil.ContainsFinalizer(dbUser, userFinalizer) {
			controllerutil.AddFinalizer(dbUser, userFinalizer)
			err = r.Update(ctx, dbUser)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if markedToBeDeleted {
			if controllerutil.ContainsFinalizer(dbUser, userFinalizer) {
				// Run finalization logic for memcachedFinalizer. If the
				// finalization logic fails, don't remove the finalizer so
				// that we can retry during the next reconciliation.
				if err := r.finalizeUser(r.Log, dbUser, ctx); err != nil {
					return ctrl.Result{}, err
				}

				// Remove memcachedFinalizer. Once all finalizers have been
				// removed, the object will be deleted.
				controllerutil.RemoveFinalizer(dbUser, userFinalizer)
				err := r.Update(ctx, dbUser)
				if err != nil {
					return ctrl.Result{}, err
				}
			}
			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *UserReconciler) finalizeUser(log logr.Logger, dbUser *dboperatorv1alpha1.User, ctx context.Context) error {
	log.Info(fmt.Sprintf("finalizing user %s", dbUser.Spec.UserName))
	pgDbServer, err := GetDbConnectionFromUser(r.Log, r.Client, ctx, dbUser)
	if err != nil {
		return err
	}
	defer pgDbServer.Close()

	err = DropPgUser(dbUser.Spec.UserName, pgDbServer)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("Failed to create user %s", dbUser.Spec.UserName))
		return err
	}
	log.Info(fmt.Sprintf("finalized user %s", dbUser.Spec.UserName))
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.User{}).
		Complete(r)
}
