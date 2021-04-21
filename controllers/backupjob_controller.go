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

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	machineryErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

func (r *BackupJobReco) GetBackupTarget() (*dboperatorv1alpha1.BackupTarget, error) {
	r.Log.Info(fmt.Sprintf("loading backupTarget %s", r.backupJob.Spec.BackupTarget))
	backupTarget := &dboperatorv1alpha1.BackupTarget{}
	nsName := types.NamespacedName{
		Name:      r.backupJob.Spec.BackupTarget,
		Namespace: r.nsNm.Namespace,
	}
	err := r.client.Get(r.ctx, nsName, backupTarget)
	return backupTarget, err
}

func (r *BackupJobReco) GetDb(dbName string) (*dboperatorv1alpha1.Db, error) {
	r.Log.Info(fmt.Sprintf("loading db %s", dbName))
	db := &dboperatorv1alpha1.Db{}
	nsName := types.NamespacedName{
		Name:      dbName,
		Namespace: r.nsNm.Namespace,
	}
	err := r.client.Get(r.ctx, nsName, db)
	return db, err
}

func (r *BackupJobReco) GetDbServer(db *dboperatorv1alpha1.Db) (*dboperatorv1alpha1.DbServer, error) {
	r.Log.Info(fmt.Sprintf("loading dbServer %s", db.Spec.Server))
	dbServer := &dboperatorv1alpha1.DbServer{}
	nsName := types.NamespacedName{
		Name:      db.Spec.Server,
		Namespace: r.nsNm.Namespace,
	}
	err := r.client.Get(r.ctx, nsName, dbServer)
	return dbServer, err
}

func (r *BackupJobReco) EnsureScripts() error {
	r.Log.Info("Ensure scripts")
	cm := &v1.ConfigMap{}
	nsName := types.NamespacedName{
		Name:      SCRIPTS_CONFIGMAP,
		Namespace: r.nsNm.Namespace,
	}
	err := r.client.Get(r.ctx, nsName, cm)
	found := true

	if err != nil {
		if machineryErrors.IsNotFound(err) {
			found = false
		} else {
			r.Log.Error(err, "Unable to lookup scripts CM")
			return err
		}
	}

	if found {
		if reflect.DeepEqual(cm.Data, SCRIPTS_MAP) {
			r.Log.Info("Scripts existed and were up to date")
			return nil
		} else {
			r.client.Delete(r.ctx, cm)
			cm = &v1.ConfigMap{}
		}
	}

	cm.Data = SCRIPTS_MAP
	cm.Name = nsName.Name
	cm.Namespace = nsName.Namespace

	r.Log.Info("Creating scripts cm")
	err = r.client.Create(r.ctx, cm)
	if err != nil {
		r.Log.Error(err, "Failed creating cm")
		return fmt.Errorf("Failed creating configmap with scripts")
	}
	return nil
}

func (r *BackupJobReco) GetS3Storage(backupTarget *dboperatorv1alpha1.BackupTarget) (dboperatorv1alpha1.S3Storage, error) {
	s3Storage := &dboperatorv1alpha1.S3Storage{}
	nsName := types.NamespacedName{
		Name:      backupTarget.Spec.StorageLocation,
		Namespace: r.nsNm.Namespace,
	}

	err := r.client.Get(r.ctx, nsName, s3Storage)
	return *s3Storage, err
}

func (r *BackupJobReco) CreateObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("creating backupJob %s", r.backupJob.Name))
	backupTarget, err := r.GetBackupTarget()
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
