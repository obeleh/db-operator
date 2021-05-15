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
	batchv1beta "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
)

// DbCopyCronJobReconciler reconciles a DbCopyCronJob object
type DbCopyCronJobReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbcopycronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbcopycronjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbcopycronjobs/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete

type DbCopyCronJobReco struct {
	Reco
	copyCronJob  dboperatorv1alpha1.DbCopyCronJob
	copyCronJobs map[string]batchv1beta.CronJob
}

func (r *DbCopyCronJobReco) MarkedToBeDeleted() bool {
	return r.copyCronJob.GetDeletionTimestamp() != nil
}

func (r *DbCopyCronJobReco) LoadObj() (bool, error) {
	r.Log.Info(fmt.Sprintf("loading copyJob %s", r.copyCronJob.Name))

	var err error
	r.copyCronJobs, err = r.GetCronJobMap()
	if err != nil {
		return false, nil
	}

	_, exists := r.copyCronJobs[r.copyCronJob.Name]
	r.Log.Info(fmt.Sprintf("copyJob %s exists: %t", r.copyCronJob.Name, exists))
	return exists, nil
}

func (r *DbCopyCronJobReco) CreateObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("creating copyJob %s", r.copyCronJob.Name))

	err := r.EnsureScripts()
	if err != nil {
		return ctrl.Result{}, err
	}

	fromDbInfo, err := r.GetDbInfo(r.copyCronJob.Spec.FromDbName)
	if err != nil {
		return ctrl.Result{}, err
	}
	toDbInfo, err := r.GetDbInfo(r.copyCronJob.Spec.ToDbName)
	if err != nil {
		return ctrl.Result{}, err
	}

	backupContainer := fromDbInfo.BuildBackupContainer()
	restoreContainer := toDbInfo.BuildRestoreContainer()
	cronJob := r.BuildCronJob([]v1.Container{backupContainer}, restoreContainer, r.copyCronJob.Name, r.copyCronJob.Spec.Interval)

	err = r.client.Create(r.ctx, &cronJob)
	if err != nil {
		r.LogError(err, "Failed to create copy cronJob")
	}
	return ctrl.Result{}, nil
}

func (r *DbCopyCronJobReco) RemoveObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Removing copyCronJob %s", r.copyCronJob.Name))
	job := &batchv1beta.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.copyCronJob.Name,
			Namespace: r.nsNm.Namespace,
		},
	}
	err := r.client.Delete(r.ctx, job)
	return ctrl.Result{}, err
}

func (r *DbCopyCronJobReco) LoadCR() (ctrl.Result, error) {
	err := r.client.Get(r.ctx, r.nsNm, &r.copyCronJob)
	if err != nil {
		r.Log.Info(fmt.Sprintf("%T: %s does not retrieved %s", r.copyCronJob, r.nsNm.Name, err))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *DbCopyCronJobReco) GetCR() client.Object {
	return &r.copyCronJob
}

func (r *DbCopyCronJobReco) EnsureCorrect() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *DbCopyCronJobReco) CleanupConn() {
}

func (r *DbCopyCronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("dbcopycronjob", req.NamespacedName)

	cr := BackupCronJobReco{
		Reco: Reco{r.Client, ctx, r.Log, req.NamespacedName},
	}
	return cr.Reco.Reconcile((&cr))
}

// SetupWithManager sets up the controller with the Manager.
func (r *DbCopyCronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.DbCopyCronJob{}).
		Complete(r)
}
