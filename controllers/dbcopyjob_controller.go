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
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
)

// DbCopyJobReconciler reconciles a DbCopyJob object
type DbCopyJobReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

type DbCopyJobReco struct {
	Reco
	copyJob  dboperatorv1alpha1.DbCopyJob
	copyJobs map[string]batchv1.Job
}

func (r *DbCopyJobReco) MarkedToBeDeleted() bool {
	return r.copyJob.GetDeletionTimestamp() != nil
}

func (r *DbCopyJobReco) LoadObj() (bool, error) {
	r.Log.Info(fmt.Sprintf("loading copyJob %s", r.copyJob.Name))

	var err error
	r.copyJobs, err = r.GetJobMap()
	if err != nil {
		return false, nil
	}

	_, exists := r.copyJobs[r.copyJob.Name]
	r.Log.Info(fmt.Sprintf("copyJob %s exists: %t", r.copyJob.Name, exists))
	return exists, nil
}

func (r *DbCopyJobReco) CreateObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("creating copyJob %s", r.copyJob.Name))

	fromDbInfo, err := r.GetDbInfo(r.copyJob.Spec.FromDbName)
	if err != nil {
		return ctrl.Result{}, err
	}
	toDbInfo, err := r.GetDbInfo(r.copyJob.Spec.ToDbName)
	if err != nil {
		return ctrl.Result{}, err
	}

	backupContainer := fromDbInfo.BuildBackupContainer()
	restoreContainer := toDbInfo.BuildRestoreContainer()

	job := r.BuildJob([]v1.Container{backupContainer}, restoreContainer, r.copyJob.Name)

	err = r.client.Create(r.ctx, &job)
	if err != nil {
		r.LogError(err, "Failed to create copy job")
	}
	return ctrl.Result{}, nil
}

func (r *DbCopyJobReco) RemoveObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Removing copyJob %s", r.copyJob.Name))
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.copyJob.Name,
			Namespace: r.nsNm.Namespace,
		},
	}
	err := r.client.Delete(r.ctx, job)
	return ctrl.Result{}, err
}

func (r *DbCopyJobReco) LoadCR() (ctrl.Result, error) {
	err := r.client.Get(r.ctx, r.nsNm, &r.copyJob)
	if err != nil {
		r.Log.Info(fmt.Sprintf("%T: %s does not exist", r.copyJob, r.nsNm.Name))
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

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbcopyjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbcopyjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=dbcopyjobs/finalizers,verbs=update
func (r *DbCopyJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("dbcopyjob", req.NamespacedName)

	rr := DbCopyJobReco{
		Reco: Reco{r.Client, ctx, r.Log, req.NamespacedName},
	}
	return rr.Reco.Reconcile((&rr))
}

// SetupWithManager sets up the controller with the Manager.
func (r *DbCopyJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.DbCopyJob{}).
		Complete(r)
}
