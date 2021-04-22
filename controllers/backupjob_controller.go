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

// BackupJobReconciler reconciles a BackupJob object
type BackupJobReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

type BackupJobReco struct {
	Reco
	backupJob  dboperatorv1alpha1.BackupJob
	backupJobs map[string]batchv1.Job
}

const SCRIPTS_CONFIGMAP string = "db-operator-scripts"

func (r *BackupJobReco) MarkedToBeDeleted() bool {
	return r.backupJob.GetDeletionTimestamp() != nil
}

func (r *BackupJobReco) LoadObj() (bool, error) {
	r.Log.Info(fmt.Sprintf("loading backupJob %s", r.backupJob.Name))
	var err error
	jobs := &batchv1.JobList{}
	opts := []client.ListOption{
		client.InNamespace(r.nsNm.Namespace),
		client.MatchingLabels{"controlledBy": "DbOperator"},
	}
	err = r.client.List(r.ctx, jobs, opts...)
	if err != nil {
		r.Log.Error(err, "failed listing Jobs")
		return false, err
	}
	r.backupJobs = make(map[string]batchv1.Job)
	for _, job := range jobs.Items {
		r.Log.Info(fmt.Sprintf("Found job %s", job.Name))
		r.backupJobs[job.Name] = job
	}
	_, exists := r.backupJobs[r.backupJob.Name]
	r.Log.Info(fmt.Sprintf("backupJob %s exists: %t", r.backupJob.Name, exists))
	return exists, nil
}

func (r *BackupJobReco) CreateObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("creating backupJob %s", r.backupJob.Name))
	backupTarget, err := r.GetBackupTarget(r.backupJob.Spec.BackupTarget)
	if err != nil {
		return ctrl.Result{}, err
	}
	db, err := r.GetDb(backupTarget.Spec.DbName)
	if err != nil {
		return ctrl.Result{}, err
	}
	dbServer, err := r.GetDbServer(db)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.EnsureScripts()
	if err != nil {
		return ctrl.Result{}, err
	}

	s3Storage, err := r.GetS3Storage(backupTarget)
	if err != nil {
		return ctrl.Result{}, err
	}

	backupContainer := BuildPostgresBackupContainer(dbServer, db)
	uploadContainer := BuildS3UploadContainer(s3Storage)

	backupPodSpec := v1.PodSpec{
		InitContainers: []v1.Container{
			backupContainer,
		},
		Containers: []v1.Container{
			uploadContainer,
		},
		RestartPolicy: v1.RestartPolicyNever,
		Volumes:       GetVolumes(),
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.backupJob.Name,
			Namespace: r.nsNm.Namespace,
			Labels: map[string]string{
				"controlledBy": "DbOperator",
			},
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: backupPodSpec,
			},
		},
	}

	err = r.client.Create(r.ctx, job)
	if err != nil {
		r.Log.Error(err, "Failed to create backup job")
	}
	return ctrl.Result{}, nil
}

func (r *BackupJobReco) RemoveObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Removing BackupJob %s", r.backupJob.Name))
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.backupJob.Name,
			Namespace: r.nsNm.Namespace,
		},
	}
	err := r.client.Delete(r.ctx, job)
	return ctrl.Result{}, err
}

func (r *BackupJobReco) LoadCR() (ctrl.Result, error) {
	err := r.client.Get(r.ctx, r.nsNm, &r.backupJob)
	if err != nil {
		r.Log.Info(fmt.Sprintf("%T: %s does not exist", r.backupJob, r.nsNm.Name))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *BackupJobReco) GetCR() client.Object {
	return &r.backupJob
}

func (r *BackupJobReco) EnsureCorrect() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *BackupJobReco) CleanupConn() {
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=backupjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=backupjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=backupjobs/finalizers,verbs=update
func (r *BackupJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("backupjob", req.NamespacedName)

	br := BackupJobReco{
		Reco: Reco{r.Client, ctx, r.Log, req.NamespacedName},
	}
	return br.Reco.Reconcile((&br))
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.BackupJob{}).
		Complete(r)
}
