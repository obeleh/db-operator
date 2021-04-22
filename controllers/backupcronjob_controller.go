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

type BackupCronJobReco struct {
	Reco
	backupCronJob  dboperatorv1alpha1.BackupJob
	backupCronJobs map[string]batchv1beta.CronJob
}

func (r *BackupCronJobReco) MarkedToBeDeleted() bool {
	return r.backupCronJob.GetDeletionTimestamp() != nil
}

func (r *BackupCronJobReco) LoadObj() (bool, error) {
	r.Log.Info(fmt.Sprintf("loading backupCronJob %s", r.backupCronJob.Name))
	var err error
	cronJobs := &batchv1beta.CronJobList{}
	opts := []client.ListOption{
		client.InNamespace(r.nsNm.Namespace),
		client.MatchingLabels{"controlledBy": "DbOperator"},
	}
	err = r.client.List(r.ctx, cronJobs, opts...)
	if err != nil {
		r.Log.Error(err, "failed listing CronJobs")
		return false, err
	}
	r.backupCronJobs = make(map[string]batchv1beta.CronJob)
	for _, cronJob := range cronJobs.Items {
		r.Log.Info(fmt.Sprintf("Found job %s", cronJob.Name))
		r.backupCronJobs[cronJob.Name] = cronJob
	}
	_, exists := r.backupCronJobs[r.backupCronJob.Name]
	r.Log.Info(fmt.Sprintf("backupCronJob %s exists: %t", r.backupCronJob.Name, exists))
	return exists, nil
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=backupcronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=backupcronjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=backupcronjobs/finalizers,verbs=update
func (r *BackupCronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("backupcronjob", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupCronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.BackupCronJob{}).
		Complete(r)
}
