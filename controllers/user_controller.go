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

	"github.com/obeleh/db-operator/shared"
	"github.com/sethvargo/go-password/password"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Log    *zap.Logger
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

func (r *UserReco) LoadObj() (bool, error) {
	var err error
	dbServer, err := GetDbServer(r.user.Spec.DbServerName, r.Client, r.NsNm.Namespace)
	if err != nil {
		return false, err
	}

	grantorUserNames := GetGrantorNamesFromDbPrivs(r.user.Spec.DbPrivs)
	conn, err := r.GetDbConnection(dbServer, grantorUserNames, nil)
	if err != nil {
		errStr := err.Error()
		if !strings.Contains(errStr, "failed getting password failed to get secret") {
			r.LogError(err, "failed getting dbInfo")
		}
		return false, err
	}
	r.conn = conn
	users, err := r.conn.GetUsers()
	if err != nil {
		return false, err
	}
	r.users = users

	_, exists := r.users[r.user.Spec.UserName]
	return exists, nil
}

func (r *UserReco) generateSecret() error {
	secretName := types.NamespacedName{
		Name:      r.user.Spec.SecretName,
		Namespace: r.user.Namespace,
	}
	secret := &v1.Secret{}
	err := r.Client.Get(r.Ctx, secretName, secret)
	// This should have given an error that it didn't exist
	// If it exists, we're happy with it, probably generated in a previous reconciliation loop
	if err == nil {
		return nil
	}

	generatedPassword, err := password.Generate(26, 9, 0, false, false)
	if err != nil {
		return err
	}
	passwordKey := "password" // nosemgrep: gitlab.gosec.G101-1
	if r.user.Spec.PasswordKey != "" {
		passwordKey = r.user.Spec.PasswordKey
	}

	secret = &v1.Secret{
		Data: map[string][]byte{
			passwordKey: []byte(generatedPassword),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.user.Spec.SecretName,
			Namespace: r.user.Namespace,
		},
	}
	return r.Client.Create(context.TODO(), secret)
}

func (r *UserReco) CreateObj() (ctrl.Result, error) {
	if r.user.Spec.GenerateSecret {
		err := r.generateSecret()
		if err != nil {
			r.LogError(err, "failed generating secret")
			return shared.GradualBackoffRetry(r.user.GetCreationTimestamp().Time), err
		}
	}

	creds, err := GetUserCredentials(&r.user, r.Client, r.Ctx)
	if err != nil {
		return r.LogAndBackoffCreation(err, r.GetCR())
	}
	r.Log.Info(fmt.Sprintf("Creating user %s", r.user.Spec.UserName))
	err = r.conn.CreateUser(r.user.Spec.UserName, *creds.Password)
	if err != nil {
		return r.LogAndBackoffCreation(err, r.GetCR())
	}
	r.NotifyChanges() // Would be nice if we could make sure the notify is only triggered once here
	res, err := r.EnsureCorrect()
	if err != nil {
		return r.LogAndBackoffCreation(err, r.GetCR())
	}
	return res, nil
}

func (r *UserReco) RemoveObj() (ctrl.Result, error) {
	if r.user.Spec.DropOnDeletion {
		r.Log.Info(fmt.Sprintf("Dropping user %s", r.user.Spec.UserName))
		err := r.conn.DropUser(r.user.Spec)
		if err != nil {
			return r.LogAndBackoffDeletion(err, r.GetCR())
		}
		r.Log.Info(fmt.Sprintf("finalized user %s", r.user.Spec.UserName))
	}
	r.NotifyChanges()
	return ctrl.Result{}, nil
}

func (r *UserReco) LoadCR() (ctrl.Result, error) {
	err := r.Client.Get(r.Ctx, r.NsNm, &r.user)
	if err != nil {
		r.Log.Info(fmt.Sprintf("%T: %s does not exist", r.user, r.NsNm.Name))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *UserReco) GetCR() client.Object {
	return &r.user
}

func (r *UserReco) EnsureCorrect() (ctrl.Result, error) {
	/*
		// A BIT UNSURE IF WE SHOULD USE THE RESOLVED DB NAME OR DB NAME IN PG CLUSTER as parameter to UpdateUserPrivs, it should already be determined by where this user lives
		errors := []error{}
		resolvedDbNamePrivs := []dboperatorv1alpha1.DbPriv{}
		for _, dbPriv := range r.user.Spec.DbPrivs {
			db := dboperatorv1alpha1.Db{}
			dbName, err := r.conn.ScopeToDbName(dbPriv.Scope)
			if err != nil {
				return false, err
			}
			nsm := types.NamespacedName{
				Name:      dbName,
				Namespace: r.user.Namespace,
			}
			err = r.Client.Get(r.Ctx, nsm, &db)
			if err == nil {
				resolvedDbNamePrivs = append(resolvedDbNamePrivs, dboperatorv1alpha1.DbPriv{
					Scope: dbPriv.Scope,
					Privs: dbPriv.Privs,
				})
			} else {
				if shared.CannotFindError(err, r.Log, "DB", r.user.Namespace, dbName) {
					err = shared.NewAlreadyHandledError(err)
				} else {
					r.LogError(err, "Failed loading DB")
				}
				errors = append(errors, err)
			}
		}
	*/
	changes, err := r.conn.UpdateUserPrivs(r.user.Spec.UserName, r.user.Spec.ServerPrivs, r.user.Spec.DbPrivs)
	if err != nil {
		return r.LogAndBackoffCreation(err, r.GetCR())
	}
	if changes {
		r.NotifyChanges()
	}
	return ctrl.Result{}, nil
}

func (r *UserReco) CleanupConn() {
	if r.conn != nil {
		r.conn.Close()
	}
}

func (r *UserReco) NotifyChanges() {
	r.Log.Info("Notifying of User changes")
	// getting dbServer because we need to figure out in what namespace it lives
	dbServer, err := GetDbServer(r.user.Spec.DbServerName, r.Client, r.user.Namespace)
	if err != nil {
		r.LogError(err, "failed notifying DBServer")
	}
	TriggerDbServerReconcile(dbServer)
}

func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.With(zap.String("Namespace", req.Namespace)).With(zap.String("Name", req.Name))
	ur := UserReco{
		Reco: Reco{shared.K8sClient{Client: r.Client, Ctx: ctx, NsNm: req.NamespacedName, Log: log}},
	}
	return ur.Reco.Reconcile(&ur)
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.User{}).
		Complete(r)
}

func GetGrantorNamesFromDbPrivs(privs []dboperatorv1alpha1.DbPriv) []string {
	userNames := []string{}
	for _, priv := range privs {
		if priv.Grantor != nil {
			userNames = append(userNames, *priv.Grantor)
		}
	}
	return userNames
}

func GetUserCredentials(dbUser *dboperatorv1alpha1.User, k8sClient client.Client, ctx context.Context) (*shared.Credentials, error) {
	if dbUser.Spec.SecretName == "" {
		if dbUser.Spec.PasswordKey != "" || dbUser.Spec.TlsCrtKey != "" || dbUser.Spec.TlsKeyKey != "" || dbUser.Spec.CaCertKey != "" {
			return nil, fmt.Errorf("SecretName is not allowed to be empty if one of these is set: password_key, ca_cert_key, tls_cert_key, tls_key_key")
		}
		return nil, nil
	}

	secretName := types.NamespacedName{
		Name:      dbUser.Spec.SecretName,
		Namespace: dbUser.Namespace,
	}
	secret := &v1.Secret{}
	err := k8sClient.Get(ctx, secretName, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %s", dbUser.Spec.SecretName)
	}

	creds := shared.Credentials{
		UserName: dbUser.Spec.UserName,
	}

	passBytes, found := secret.Data[shared.Nvl(dbUser.Spec.PasswordKey, "password")]
	if found {
		password := string(passBytes)
		creds.Password = &password
	}

	keys := []struct {
		specKey  *string
		credsKey **string
	}{
		{&dbUser.Spec.CaCertKey, &creds.CaCert},
		{&dbUser.Spec.TlsKeyKey, &creds.TlsKey},
		{&dbUser.Spec.TlsCrtKey, &creds.TlsCrt},
	}

	for _, key := range keys {
		if *key.specKey != "" {
			valueBytes, found := secret.Data[*key.specKey]
			if !found {
				return nil, fmt.Errorf("key '%s' not found in secret %s.%s", *key.specKey, dbUser.Namespace, dbUser.Spec.SecretName)
			}
			value := string(valueBytes)
			*key.credsKey = &value
		}
	}

	return &creds, nil
}
