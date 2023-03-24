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

// DbCopyCronJobReconciler reconciles a DbCopyCronJob object
type DbCopyCronJobReconciler struct {
	client.Client
	Log    *zap.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbcopycronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbcopycronjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbcopycronjobs/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete

type DbCopyCronJobReco struct {
	Reco
	copyCronJob  dboperatorv1alpha1.DbCopyCronJob
	copyCronJobs map[string]batchv1.CronJob
	StatusWriter client.StatusWriter
}

func (r *DbCopyCronJobReco) MarkedToBeDeleted() bool {
	return r.copyCronJob.GetDeletionTimestamp() != nil
}

func (r *DbCopyCronJobReco) LoadObj() (bool, error) {
	r.Log.Info(fmt.Sprintf("loading copyCronJob %s", r.copyCronJob.Name))

	var err error
	r.copyCronJobs, err = r.GetCronJobMap()
	if err != nil {
		if !shared.CannotFindError(err, r.Log, "CopyCronJob", r.nsNm.Namespace, r.nsNm.Name) {
			r.LogError(err, "failed getting CopyCronJob")
			return false, err
		}
		return false, nil
	}

	_, exists := r.copyCronJobs[r.copyCronJob.Name]
	r.Log.Info(fmt.Sprintf("copyCronJob %s exists: %t", r.copyCronJob.Name, exists))
	r.UpdateStatus(exists)
	return exists, nil
}

func (r *DbCopyCronJobReco) UpdateStatus(exists bool) {
	newStatus := dboperatorv1alpha1.DbCopyCronJobStatus{
		Exists:      exists,
		CronJobName: r.copyCronJob.Name,
	}
	if !reflect.DeepEqual(r.copyCronJob.Status, newStatus) {
		r.copyCronJob.Status = newStatus
		r.StatusWriter.Update(r.ctx, &r.copyCronJob)
	}
}

func (r *DbCopyCronJobReco) CreateObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("creating copyCronJob %s", r.copyCronJob.Name))

	err := r.EnsureScripts()
	if err != nil {
		return ctrl.Result{}, err
	}

	fromDbServerActions, err := r.GetServerActionsFromDbName(r.copyCronJob.Spec.FromDbName)
	if err != nil {
		return ctrl.Result{}, err
	}
	toDbServerActions, err := r.GetServerActionsFromDbName(r.copyCronJob.Spec.ToDbName)
	if err != nil {
		return ctrl.Result{}, err
	}

	backupContainer := fromDbServerActions.BuildBackupContainer()
	restoreContainer := toDbServerActions.BuildRestoreContainer()
	cronJob := r.BuildCronJob(
		[]v1.Container{backupContainer},
		restoreContainer,
		r.copyCronJob.Name,
		r.copyCronJob.Spec.Interval,
		r.copyCronJob.Spec.Suspend,
		r.copyCronJob.Spec.ServiceAccount,
	)

	err = r.client.Create(r.ctx, &cronJob)
	if err != nil && !shared.AlreadyExistsError(err, r.Log, cronJob.Kind, cronJob.Namespace, cronJob.Name) {
		r.LogError(err, "Failed to create copy cronJob")
	}
	r.UpdateStatus(true)
	return ctrl.Result{}, nil
}

func (r *DbCopyCronJobReco) RemoveObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Removing copyCronJob %s", r.copyCronJob.Name))
	job := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.copyCronJob.Name,
			Namespace: r.nsNm.Namespace,
		},
	}
	err := r.client.Delete(r.ctx, job)
	r.UpdateStatus(false)
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

func (r *DbCopyCronJobReco) EnsureCorrect() (bool, error) {
	return true, nil
}

func (r *DbCopyCronJobReco) CleanupConn() {
}

func (r *DbCopyCronJobReco) NotifyChanges() {
}

func (r *DbCopyCronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.With(zap.String("Namespace", req.Namespace)).With(zap.String("Name", req.Name))

	cr := DbCopyCronJobReco{
		Reco:         Reco{r.Client, ctx, log, req.NamespacedName},
		StatusWriter: r.Status(),
	}
	return cr.Reco.Reconcile((&cr))
}

// SetupWithManager sets up the controller with the Manager.
func (r *DbCopyCronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.DbCopyCronJob{}).
		Complete(r)
}
