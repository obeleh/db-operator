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

// RestoreJobReconciler reconciles a RestoreJob object
type RestoreJobReconciler struct {
	client.Client
	Log    *zap.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=restorejobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=restorejobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=restorejobs/finalizers,verbs=update

type RestoreJobReco struct {
	Reco
	restoreJob  dboperatorv1alpha1.RestoreJob
	restoreJobs map[string]batchv1.Job
}

func (r *RestoreJobReco) MarkedToBeDeleted() bool {
	return r.restoreJob.GetDeletionTimestamp() != nil
}

func (r *RestoreJobReco) LoadObj() (bool, error) {
	r.Log.Info(fmt.Sprintf("loading restoreJob %s", r.restoreJob.Name))

	var err error
	r.restoreJobs, err = r.GetJobMap()
	if err != nil {
		if !shared.CannotFindError(err, r.Log, "DbServer", r.NsNm.Namespace, r.NsNm.Name) {
			r.LogError(err, "failed getting DbServer")
			return false, err
		}
		return false, nil
	}

	_, exists := r.restoreJobs[r.restoreJob.Name]
	r.Log.Info(fmt.Sprintf("restoreJob %s exists: %t", r.restoreJob.Name, exists))
	return exists, nil
}

func (r *RestoreJobReco) CreateObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("creating restoreJob %s", r.restoreJob.Name))

	err := r.EnsureScripts()
	if err != nil {
		return ctrl.Result{}, err
	}

	restoreTarget, err := r.GetRestoreTarget(r.restoreJob.Spec.RestoreTarget)
	if err != nil {
		return ctrl.Result{}, err
	}
	actions, err := r.GetServerActionsFromDbName(restoreTarget.Spec.DbName)
	if err != nil {
		return ctrl.Result{}, err
	}
	storageInfo, err := r.GetStorageActions(restoreTarget.Spec.StorageType, restoreTarget.Spec.StorageLocation)
	if err != nil {
		return ctrl.Result{}, err
	}

	restoreContainer := actions.BuildRestoreContainer()
	downloadContainer := storageInfo.BuildDownloadContainer(r.restoreJob.Spec.FixedFileName)

	job := r.BuildJob([]v1.Container{downloadContainer}, restoreContainer, r.restoreJob.Name, r.restoreJob.Spec.ServiceAccount)

	err = r.Client.Create(r.Ctx, &job)
	if err != nil && !shared.AlreadyExistsError(err, r.Log, job.Kind, job.Namespace, job.Name) {
		r.LogError(err, "Failed to create restore job")
	}
	return ctrl.Result{}, nil
}

func (r *RestoreJobReco) RemoveObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Removing restoreJob %s", r.restoreJob.Name))
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.restoreJob.Name,
			Namespace: r.NsNm.Namespace,
		},
	}
	err := r.Client.Delete(r.Ctx, job)
	return ctrl.Result{}, err
}

func (r *RestoreJobReco) LoadCR() (ctrl.Result, error) {
	err := r.Client.Get(r.Ctx, r.NsNm, &r.restoreJob)
	if err != nil {
		r.Log.Info(fmt.Sprintf("%T: %s does not exist", r.restoreJob, r.NsNm.Name))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *RestoreJobReco) GetCR() client.Object {
	return &r.restoreJob
}

func (r *RestoreJobReco) EnsureCorrect() (bool, ctrl.Result, error) {
	return false, ctrl.Result{}, nil
}

func (r *RestoreJobReco) CleanupConn() {
}

func (r *RestoreJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.With(zap.String("Namespace", req.Namespace)).With(zap.String("Name", req.Name))

	rr := RestoreJobReco{
		Reco: Reco{shared.K8sClient{Client: r.Client, Ctx: ctx, NsNm: req.NamespacedName, Log: log}},
	}
	return rr.Reco.Reconcile((&rr))
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.RestoreJob{}).
		Complete(r)
}
