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

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
)

// RestoreCronJobReconciler reconciles a RestoreCronJob object
type RestoreCronJobReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=restorecronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=restorecronjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=restorecronjobs/finalizers,verbs=update

type RestoreCronJobReco struct {
	Reco
	restoreCronJob  dboperatorv1alpha1.RestoreCronJob
	restoreCronJobs map[string]batchv1beta.CronJob
}

func (r *RestoreCronJobReco) MarkedToBeDeleted() bool {
	return r.restoreCronJob.GetDeletionTimestamp() != nil
}

func (r *RestoreCronJobReco) LoadObj() (bool, error) {
	r.Log.Info(fmt.Sprintf("loading restoreCronJob %s", r.restoreCronJob.Name))
	var err error
	r.restoreCronJobs, err = r.GetCronJobMap()
	if err != nil {
		return false, nil
	}

	_, exists := r.restoreCronJobs[r.restoreCronJob.Name]
	r.Log.Info(fmt.Sprintf("restoreCronJob %s exists: %t", r.restoreCronJob.Name, exists))
	return exists, nil
}

func (r *RestoreCronJobReco) CreateObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("creating restoreCronJob %s", r.restoreCronJob.Name))

	err := r.EnsureScripts()
	if err != nil {
		return ctrl.Result{}, err
	}

	storageInfo, dbInfo, err := r.GetRestoreTargetFull(r.restoreCronJob.Spec.RestoreTarget)
	if err != nil {
		return ctrl.Result{}, err
	}

	restoreContainer := dbInfo.BuildRestoreContainer()
	downloadContainer := storageInfo.BuildDownloadContainer(r.restoreCronJob.Spec.FixedFileName)
	cronJob := r.BuildCronJob([]v1.Container{downloadContainer}, restoreContainer, r.restoreCronJob.Name, r.restoreCronJob.Spec.Interval, r.restoreCronJob.Spec.Suspend)

	err = r.client.Create(r.ctx, &cronJob)
	if err != nil {
		r.LogError(err, "Failed to create restore cronjob")
	}
	return ctrl.Result{}, nil
}

func (r *RestoreCronJobReco) RemoveObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Removing restoreCronJob %s", r.restoreCronJob.Name))
	cronJob := &batchv1beta.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.restoreCronJob.Name,
			Namespace: r.nsNm.Namespace,
		},
	}
	err := r.client.Delete(r.ctx, cronJob)
	return ctrl.Result{}, err
}

func (r *RestoreCronJobReco) LoadCR() (ctrl.Result, error) {
	err := r.client.Get(r.ctx, r.nsNm, &r.restoreCronJob)
	if err != nil {
		r.Log.Info(fmt.Sprintf("%T: %s does not exist", r.restoreCronJob, r.nsNm.Name))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *RestoreCronJobReco) GetCR() client.Object {
	return &r.restoreCronJob
}

func (r *RestoreCronJobReco) EnsureCorrect() (bool, error) {
	return false, nil
}

func (r *RestoreCronJobReco) CleanupConn() {
}

func (r *RestoreCronJobReco) NotifyChanges() {
}

func (r *RestoreCronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("restorecronjob", req.NamespacedName)

	rr := RestoreCronJobReco{
		Reco: Reco{r.Client, ctx, r.Log, req.NamespacedName},
	}
	return rr.Reco.Reconcile((&rr))
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreCronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.RestoreCronJob{}).
		Complete(r)
}
