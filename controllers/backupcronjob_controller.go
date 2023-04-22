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
	"reflect"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/shared"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BackupCronJobReconciler reconciles a BackupCronJob object
type BackupCronJobReconciler struct {
	client.Client
	Log    *zap.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=backupcronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=backupcronjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=backupcronjobs/finalizers,verbs=update

type BackupCronJobReco struct {
	Reco
	backupCronJob  dboperatorv1alpha1.BackupCronJob
	backupCronJobs map[string]batchv1.CronJob
	StatusWriter   client.StatusWriter
}

func (r *BackupCronJobReco) MarkedToBeDeleted() bool {
	return r.backupCronJob.GetDeletionTimestamp() != nil
}

func (r *BackupCronJobReco) LoadObj() (bool, error) {
	r.Log.Info(fmt.Sprintf("loading backupCronJob %s", r.backupCronJob.Name))
	var err error
	r.backupCronJobs, err = r.GetCronJobMap()
	if err != nil {
		if !shared.CannotFindError(err, r.Log, "BackupCronJob", r.NsNm.Namespace, r.NsNm.Name) {
			r.LogError(err, "failed getting BackupCronJob")
			return false, err
		}
		return false, nil
	}

	_, exists := r.backupCronJobs[r.backupCronJob.Name]
	r.Log.Info(fmt.Sprintf("backupCronJob %s exists: %t", r.backupCronJob.Name, exists))
	r.UpdateStatus(exists)
	return exists, nil
}

func (r *BackupCronJobReco) UpdateStatus(exists bool) {
	newStatus := dboperatorv1alpha1.BackupCronJobStatus{
		Exists:      exists,
		CronJobName: r.backupCronJob.Name,
	}
	if !reflect.DeepEqual(r.backupCronJob.Status, newStatus) {
		r.backupCronJob.Status = newStatus
		r.StatusWriter.Update(r.Ctx, &r.backupCronJob)
	}
}

func (r *BackupCronJobReco) CreateObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("creating backupCronJob %s", r.backupCronJob.Name))

	err := r.EnsureScripts()
	if err != nil {
		return ctrl.Result{}, err
	}

	backupTarget, err := r.GetBackupTarget(r.backupCronJob.Spec.BackupTarget)
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
	uploadContainer := storageInfo.BuildUploadContainer(r.backupCronJob.Spec.FixedFileName)
	cronJob := r.BuildCronJob(
		[]v1.Container{backupContainer},
		uploadContainer,
		r.backupCronJob.Name,
		r.backupCronJob.Spec.Interval,
		r.backupCronJob.Spec.Suspend,
		r.backupCronJob.Spec.ServiceAccount,
	)

	err = r.Client.Create(r.Ctx, &cronJob)
	if err != nil && !shared.AlreadyExistsError(err, r.Log, cronJob.Kind, cronJob.Namespace, cronJob.Name) {
		r.LogError(err, "Failed to create backup cronjob")
	}
	r.UpdateStatus(true)
	return ctrl.Result{}, nil
}

func (r *BackupCronJobReco) RemoveObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Removing BackupCronJob %s", r.backupCronJob.Name))
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.backupCronJob.Name,
			Namespace: r.NsNm.Namespace,
		},
	}
	err := r.Client.Delete(r.Ctx, cronJob)
	r.UpdateStatus(false)
	return ctrl.Result{}, err
}

func (r *BackupCronJobReco) LoadCR() (ctrl.Result, error) {
	err := r.Client.Get(r.Ctx, r.NsNm, &r.backupCronJob)
	if err != nil {
		r.Log.Info(fmt.Sprintf("%T: %s does not exist", r.backupCronJob, r.NsNm.Name))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *BackupCronJobReco) GetCR() client.Object {
	return &r.backupCronJob
}

func (r *BackupCronJobReco) EnsureCorrect() (bool, error) {
	return false, nil
}

func (r *BackupCronJobReco) CleanupConn() {
}

func (r *BackupCronJobReco) NotifyChanges() {
}

func (r *BackupCronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.With(zap.String("Namespace", req.Namespace)).With(zap.String("Name", req.Name))

	br := BackupCronJobReco{
		Reco:         Reco{shared.K8sClient{r.Client, ctx, req.NamespacedName, log}},
		StatusWriter: r.Status(),
	}
	return br.Reco.Reconcile((&br))
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupCronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.BackupCronJob{}).
		Complete(r)
}
