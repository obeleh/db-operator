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

// BackupJobReconciler reconciles a BackupJob object
type BackupJobReconciler struct {
	client.Client
	Log    *zap.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=backupjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=backupjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=backupjobs/finalizers,verbs=update

type BackupJobReco struct {
	Reco
	backupJob  dboperatorv1alpha1.BackupJob
	backupJobs map[string]batchv1.Job
}

func (r *BackupJobReco) MarkedToBeDeleted() bool {
	return r.backupJob.GetDeletionTimestamp() != nil
}

func (r *BackupJobReco) LoadObj() (bool, error) {
	r.Log.Info(fmt.Sprintf("Loading backupJob %s", r.backupJob.Name))
	var err error
	r.backupJobs, err = r.GetJobMap()
	if err != nil {
		if !shared.CannotFindError(err, r.Log, "BackupJob", r.NsNm.Namespace, r.NsNm.Name) {
			r.LogError(err, "Failed getting BackupJob")
			return false, shared.NewAlreadyHandledError(err)
		}
		return false, nil
	}
	_, exists := r.backupJobs[r.backupJob.Name]
	r.Log.Info(fmt.Sprintf("BackupJob %s exists: %t", r.backupJob.Name, exists))
	return exists, nil
}

func (r *BackupJobReco) CreateObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Creating backupJob %s", r.backupJob.Name))

	err := r.EnsureScripts()
	if err != nil {
		return ctrl.Result{}, err
	}

	backupTarget, err := r.GetBackupTarget(r.backupJob.Spec.BackupTarget)
	if err != nil {
		return ctrl.Result{}, err
	}
	actions, err := r.GetServerActionsFromDbName(backupTarget.Spec.DbName)
	if err != nil {
		return ctrl.Result{}, err
	}
	storageInfo, err := r.GetStorageActions(backupTarget.Spec.StorageType, backupTarget.Spec.StorageLocation)
	if err != nil {
		return ctrl.Result{}, err
	}

	backupContainer := actions.BuildBackupContainer()
	uploadContainer := storageInfo.BuildUploadContainer(r.backupJob.Spec.FixedFileName)
	job := r.BuildJob([]v1.Container{backupContainer}, uploadContainer, r.backupJob.Name, r.backupJob.Spec.ServiceAccount)

	err = r.Client.Create(r.Ctx, &job)
	if err != nil && !shared.AlreadyExistsError(err, r.Log, job.Kind, job.Namespace, job.Name) {
		r.LogError(err, "Failed to create backup job")
	}
	return ctrl.Result{}, nil
}

func (r *BackupJobReco) RemoveObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Removing BackupJob %s", r.backupJob.Name))
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.backupJob.Name,
			Namespace: r.NsNm.Namespace,
		},
	}
	err := r.Client.Delete(r.Ctx, job)
	return ctrl.Result{}, err
}

func (r *BackupJobReco) LoadCR() (ctrl.Result, error) {
	err := r.Client.Get(r.Ctx, r.NsNm, &r.backupJob)
	if err != nil {
		r.Log.Info(fmt.Sprintf("%T: %s does not exist", r.backupJob, r.NsNm.Name))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *BackupJobReco) GetCR() client.Object {
	return &r.backupJob
}

func (r *BackupJobReco) EnsureCorrect() (bool, ctrl.Result, error) {
	return false, ctrl.Result{}, nil
}

func (r *BackupJobReco) CleanupConn() {
}

func (r *BackupJobReco) NotifyChanges() {
}

func (r *BackupJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.With(zap.String("Namespace", req.Namespace)).With(zap.String("Name", req.Name))

	br := BackupJobReco{
		Reco: Reco{shared.K8sClient{Client: r.Client, Ctx: ctx, NsNm: req.NamespacedName, Log: log}},
	}
	return br.Reco.Reconcile((&br))
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.BackupJob{}).
		Complete(r)
}
