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

	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/shared"
)

// DbCopyJobReconciler reconciles a DbCopyJob object
type DbCopyJobReconciler struct {
	client.Client
	Log    *zap.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbcopyjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbcopyjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbcopyjobs/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

type DbCopyJobReco struct {
	Reco
	copyJob  dboperatorv1alpha1.DbCopyJob
	copyJobs map[string]batchv1.Job
}

func (r *DbCopyJobReco) LoadObj() (bool, error) {
	r.Log.Info(fmt.Sprintf("loading copyJob %s", r.copyJob.Name))

	var err error
	r.copyJobs, err = r.GetJobMap()
	if err != nil {
		if !shared.CannotFindError(err, r.Log, "DbServer", r.NsNm.Namespace, r.NsNm.Name) {
			r.LogError(err, "failed getting DbServer")
			return false, err
		}
		return false, nil
	}

	_, exists := r.copyJobs[r.copyJob.Name]
	r.Log.Info(fmt.Sprintf("copyJob %s exists: %t", r.copyJob.Name, exists))
	return exists, nil
}

func (r *DbCopyJobReco) CreateObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("creating copyJob %s", r.copyJob.Name))

	err := r.EnsureScripts()
	if err != nil {
		return ctrl.Result{}, err
	}

	fromDbServerActions, err := r.GetServerActionsFromDbName(r.copyJob.Spec.FromDbName)
	if err != nil {
		return ctrl.Result{}, err
	}
	toDbServerActions, err := r.GetServerActionsFromDbName(r.copyJob.Spec.ToDbName)
	if err != nil {
		return ctrl.Result{}, err
	}

	backupContainer := fromDbServerActions.BuildBackupContainer()
	restoreContainer := toDbServerActions.BuildRestoreContainer()

	job := r.BuildJob([]v1.Container{backupContainer}, restoreContainer, r.copyJob.Name, r.copyJob.Spec.ServiceAccount)

	err = r.Client.Create(r.Ctx, &job)
	if err != nil && !shared.AlreadyExistsError(err, r.Log, job.Kind, job.Namespace, job.Name) {
		r.LogError(err, "Failed to create copy job")
	}
	return ctrl.Result{}, nil
}

func (r *DbCopyJobReco) RemoveObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Removing copyJob %s", r.copyJob.Name))
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.copyJob.Name,
			Namespace: r.NsNm.Namespace,
		},
	}
	err := r.Client.Delete(r.Ctx, job)
	if err != nil {
		return r.LogAndBackoffDeletion(err, r.GetCR())
	}
	return ctrl.Result{}, nil
}

func (r *DbCopyJobReco) LoadCR() (ctrl.Result, error) {
	err := r.Client.Get(r.Ctx, r.NsNm, &r.copyJob)
	if err != nil {
		r.Log.Info(fmt.Sprintf("%T: %s does not exist", r.copyJob, r.NsNm.Name))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *DbCopyJobReco) GetCR() client.Object {
	return &r.copyJob
}

func (r *DbCopyJobReco) EnsureCorrect() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *DbCopyJobReco) CleanupConn() {
}

func (r *DbCopyJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.With(zap.String("Namespace", req.Namespace)).With(zap.String("Name", req.Name))

	rr := DbCopyJobReco{
		Reco: Reco{shared.K8sClient{Client: r.Client, Ctx: ctx, NsNm: req.NamespacedName, Log: log}},
	}
	return rr.Reco.Reconcile((&rr))
}

// SetupWithManager sets up the controller with the Manager.
func (r *DbCopyJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.DbCopyJob{}).
		Complete(r)
}
