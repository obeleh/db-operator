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
		return false, err
	}
	r.backupJobs = make(map[string]batchv1.Job)
	for _, job := range jobs.Items {
		r.backupJobs[job.Name] = job
	}
	_, exists := r.backupJobs[r.backupJob.Name]
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
	cm := &v1.ConfigMap{}
	nsName := types.NamespacedName{
		Name:      "db-operator-scripts",
		Namespace: r.nsNm.Namespace,
	}
	err := r.client.Get(r.ctx, nsName, cm)
	if err != nil && machineryErrors.IsNotFound(err) {
		cm.Data["backup_postgres.sh"] = `
		#!/bin/bash -e
		pg_dump \
			--format=custom \
			--compress=9 \
			--file=/pgdump/${1:-"$DATABASE"_"` + "`" + "date +%Y%m%d%H%M" + "`.dump\"} \\" + `
			--no-owner \
			--no-acl \
			--host=$PGHOST \
			--user=$PGUSER \
			--port=$PGPORT \
			$DATABASE
		echo "pg_dump done"
		`
		cm.Data["restore_postgres.sh"] = `
		#!/bin/bash -e
		LATEST_DUMP=$(find /pgdump/ -type f | sort | tail -n 1)
		pg_restore \
			--host=$PGHOST \
			--user=$PGUSER \
			--port=$PGPORT \
			--dbname=$DATABASE \
			--single-transaction \
			--no-owner \
			--no-acl \
			-n public \
			$LATEST_DUMP
		echo "pg_restore done"
		`

		cm.Data["backup_az_blobs.sh"] = `
		#!/bin/bash -e
		# Only supports up/downloading with service principal
		# But should be fairly simple to expand with other methods.
		LATEST_DUMP=$(find /pgdump/ -type f | sort | tail -n 1)
		LATEST_DUMP_BASE_NAME=$(basename "$LATEST_DUMP")
		az login --service-principal -u $AZ_BLOBS_USER -p $AZ_BLOBS_USER_PW --tenant $AZ_BLOBS_TENANT_ID
		az storage blob upload \
		  --account-name $AZ_BLOBS_STORAGE_ACCOUNT \
		  --container-name $AZ_BLOBS_CONTAINER \
		  --name $LATEST_DUMP_BASE_NAME \
		  --file $LATEST_DUMP
		echo "upload done"
		`

		cm.Data["download_az_blobs.sh"] = `
		#!/bin/bash -e
		# Only supports up/downloading with service principal
		# But should be fairly simple to expand with other methods.
		az login --service-principal -u $AZ_BLOBS_USER -p $AZ_BLOBS_USER_PW --tenant $AZ_BLOBS_TENANT_ID
		az storage blob download \
		  --account-name $AZ_BLOBS_STORAGE_ACCOUNT \
		  --container-name $AZ_BLOBS_CONTAINER \
		  --name $AZ_BLOBS_FILE_NAME \
		  --file /pgdump/$AZ_BLOBS_FILE_NAME
		echo "download done"
		`

		cm.Data["backup_s3.sh"] = `
		#!/bin/bash -e
		LATEST_DUMP=$(find /pgdump/ -type f | sort | tail -n 1)
		LATEST_DUMP_BASE_NAME=$(basename "$LATEST_DUMP")
		# aws s3 cp test.txt s3://mybucket/test2.txt
		aws s3 cp $LATEST_DUMP s3://$S3_BUCKET/$S3_PREFIX
		echo "upload done"
		`

		cm.Data["download_s3.sh"] = `
		#!/bin/bash -e
		aws s3 cp s3://$S3_BUCKET/$S3_PREFIX$S3_FILE_NAME /pgdump/$S3_FILE_NAME
		echo "download done"
		`

		err = r.client.Create(r.ctx, cm)
		if err != nil {
			return fmt.Errorf("Failed creating configmap with scripts")
		}

	}

	if err != nil {
		return fmt.Errorf("Failed getting configmap with scripts")
	}

	return nil
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

	backupEnvVars := []v1.EnvVar{
		{Name: "PGHOST", Value: dbServer.Spec.Address},
		{Name: "PGUSER", Value: dbServer.Spec.UserName},
		{Name: "PGPASSWORD", ValueFrom: &v1.EnvVarSource{
			SecretKeyRef: &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: dbServer.Spec.SecretName,
				},
				Key: Nvl(dbServer.Spec.SecretKey, "password"),
			},
		}},
		{Name: "DATABASE", Value: db.Spec.DbName},
	}

	pgDumpSpec := v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:  "pg-dump",
				Image: "postgres:latest",
				Env:   backupEnvVars,
				Command: []string{
					"/scripts/backup_postgres.sh",
				},
				VolumeMounts: []v1.VolumeMount{
					{Name: "scripts", MountPath: "/scripts"},
					{Name: "pgdump", MountPath: "/pgdump"},
				},
			},
		},
		RestartPolicy: v1.RestartPolicyNever,
		Volumes:       []v1.Volume{},
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: r.backupJob.Name, Namespace: r.nsNm.Namespace},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: pgDumpSpec,
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
