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
	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
	batchv1beta "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BackupCronJobReconciler reconciles a BackupCronJob object
type BackupCronJobReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=backupcronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=backupcronjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=backupcronjobs/finalizers,verbs=update

type BackupCronJobReco struct {
	Reco
	backupCronJob  dboperatorv1alpha1.BackupCronJob
	backupCronJobs map[string]batchv1beta.CronJob
}

func (r *BackupCronJobReco) MarkedToBeDeleted() bool {
	return r.backupCronJob.GetDeletionTimestamp() != nil
}

func (r *BackupCronJobReco) LoadObj() (bool, error) {
	r.Log.Info(fmt.Sprintf("loading backupCronJob %s", r.backupCronJob.Name))
	var err error
	r.backupCronJobs, err = r.GetCronJobMap()
	if err != nil {
		return false, nil
	}

	_, exists := r.backupCronJobs[r.backupCronJob.Name]
	r.Log.Info(fmt.Sprintf("backupCronJob %s exists: %t", r.backupCronJob.Name, exists))
	return exists, nil
}

func (r *BackupCronJobReco) CreateObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("creating backupCronJob %s", r.backupCronJob.Name))

	storageInfo, dbInfo, err := r.GetBackupTargetFull(r.backupCronJob.Spec.BackupTarget)
	if err != nil {
		return ctrl.Result{}, err
	}

	backupContainer := dbInfo.BuildBackupContainer()
	uploadContainer := storageInfo.BuildUploadContainer(r.backupCronJob.Spec.FixedFileName)
	cronJob := r.BuildCronJob([]v1.Container{backupContainer}, uploadContainer, r.backupCronJob.Name, r.backupCronJob.Spec.Interval)

	err = r.client.Create(r.ctx, &cronJob)
	if err != nil {
		r.LogError(err, "Failed to create backup cronjob")
	}
	return ctrl.Result{}, nil
}

func (r *BackupCronJobReco) RemoveObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Removing BackupCronJob %s", r.backupCronJob.Name))
	cronJob := &batchv1beta.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.backupCronJob.Name,
			Namespace: r.nsNm.Namespace,
		},
	}
	err := r.client.Delete(r.ctx, cronJob)
	return ctrl.Result{}, err
}

func (r *BackupCronJobReco) LoadCR() (ctrl.Result, error) {
	err := r.client.Get(r.ctx, r.nsNm, &r.backupCronJob)
	if err != nil {
		r.Log.Info(fmt.Sprintf("%T: %s does not exist", r.backupCronJob, r.nsNm.Name))
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
	_ = r.Log.WithValues("backupCronJob", req.NamespacedName)

	br := BackupCronJobReco{
		Reco: Reco{r.Client, ctx, r.Log, req.NamespacedName},
	}
	return br.Reco.Reconcile((&br))
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupCronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.BackupCronJob{}).
		Complete(r)
}
