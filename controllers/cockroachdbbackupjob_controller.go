/*
Copyright 2022.

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
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/dbservers/postgres"
	"github.com/obeleh/db-operator/shared"
)

// CockroachDBBackupJobReconciler reconciles a CockroachDBBackupJob object
type CockroachDBBackupJobReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=cockroachdbbackupjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=cockroachdbbackupjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=cockroachdbbackupjobs/finalizers,verbs=update

type CrdbBackubJobReco struct {
	Reco
	backupJob    dboperatorv1alpha1.CockroachDBBackupJob
	StatusClient client.StatusClient
	conn         *postgres.PostgresConnection
}

func (r *CrdbBackubJobReco) MarkedToBeDeleted() bool {
	return r.backupJob.GetDeletionTimestamp() != nil
}

func (r *CrdbBackubJobReco) LoadObj() (bool, error) {
	r.Log.Info(fmt.Sprintf("loading Cockroachdb backupJob %s", r.backupJob.Name))
	var err error
	if r.backupJob.Status.JobId == 0 {
		r.Log.Info(fmt.Sprintf("backupJob %s does not have a job_id, ignoring as this job mayb have been executed without the operator recording an id", r.backupJob.Name))
		return false, nil
	}

	pgConn, err := r.getPostgresConnectionFromBackupTarget()
	if err != nil {
		return false, err
	}

	jobMap, found, err := pgConn.GetBackupJobById(r.backupJob.Status.JobId)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	r.backupJob.Status, err = jobMapToJobStatus(jobMap)
	if err != nil {
		return false, err
	}

	r.Log.Info(fmt.Sprintf("backupJob %s exists with ID: %s", r.backupJob.Name, r.backupJob.Status.JobId))
	return true, nil
}

func jobMapToJobStatus(jobMap map[string]interface{}) (dboperatorv1alpha1.CockroachDBBackupJobStatus, error) {
	return dboperatorv1alpha1.CockroachDBBackupJobStatus{
		JobId:       jobMap["job_id"].(int64),
		Status:      jobMap["status"].(string),
		Description: jobMap["description"].(string),
		Created:     jobMap["created"].(metav1.Time),
		Started:     jobMap["started"].(metav1.Time),
		Finished:    jobMap["finished"].(metav1.Time),
		Error:       jobMap["error"].(string),
	}, nil
}

func (r *CrdbBackubJobReco) GetJobMap() (map[int64]dboperatorv1alpha1.CockroachDBBackupJobStatus, error) {
	jobsMap := make(map[int64]dboperatorv1alpha1.CockroachDBBackupJobStatus)

	pgConn, err := r.getPostgresConnectionFromBackupTarget()
	if err != nil {
		return jobsMap, err
	}

	jobs, err := pgConn.GetBackupJobs()
	if err != nil {
		return jobsMap, err
	}

	for _, jobMap := range jobs {
		jobStatus, err := jobMapToJobStatus(jobMap)
		if err != nil {
			return jobsMap, fmt.Errorf("Failed to load jobmap %v", err)
		}
		r.Log.Info(fmt.Sprintf("Found job %s", jobStatus.JobId))
		jobsMap[jobStatus.JobId] = jobStatus
	}
	return jobsMap, nil
}

func (r *CrdbBackubJobReco) getPostgresConnectionFromBackupTarget() (*postgres.PostgresConnection, error) {
	if len(r.backupJob.Spec.BackupTarget) == 0 {
		return nil, fmt.Errorf("Empty backup_target for CockroachDBBackupJob %s", r.backupJob.Name)
	}
	_, dbInfo, err := r.GetBackupTargetFull(r.backupJob.Spec.BackupTarget)
	if err != nil {
		return nil, err
	}

	return r.getPostgresConnectionFromDbInfo(dbInfo)
}

func (r *CrdbBackubJobReco) getPostgresConnectionFromDbInfo(dbInfo shared.DbActions) (*postgres.PostgresConnection, error) {
	if r.conn == nil || r.conn.Conn == nil {
		conn, err := dbInfo.GetDbConnection()
		if err != nil {
			return nil, err
		}
		concreteConn := conn.(*postgres.PostgresConnection)
		r.conn = concreteConn
	}
	return r.conn, nil
}

func (r *CrdbBackubJobReco) SetStatus(backupJob *dboperatorv1alpha1.CockroachDBBackupJob, ctx context.Context, newStatus dboperatorv1alpha1.CockroachDBBackupJobStatus) error {
	if !reflect.DeepEqual(r.backupJob.Status, newStatus) {
		backupJob.Status = newStatus
		err := r.StatusClient.Status().Update(ctx, backupJob)
		if err != nil {
			message := fmt.Sprintf("failed patching status %s", err)
			r.Log.Info(message)
			return fmt.Errorf(message)
		}
	}
	return nil
}

func (r *CrdbBackubJobReco) LoadCR() (ctrl.Result, error) {
	err := r.client.Get(r.ctx, r.nsNm, &r.backupJob)
	if err != nil {
		r.Log.Info(fmt.Sprintf("%T: %s does not exist", r.backupJob, r.nsNm.Name))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *CrdbBackubJobReco) GetCR() client.Object {
	return &r.backupJob
}

func (r *CrdbBackubJobReco) EnsureCorrect() (bool, error) {
	return false, nil
}

func (r *CrdbBackubJobReco) CleanupConn() {
	if r.conn != nil {
		r.conn.Close()
	}
}

func (r *CrdbBackubJobReco) NotifyChanges() {
}

func (r *CrdbBackubJobReco) buildRetryResult() ctrl.Result {
	return ctrl.Result{
		// Gradual backoff
		Requeue:      true,
		RequeueAfter: time.Duration(time.Since(r.backupJob.GetCreationTimestamp().Time).Seconds()),
	}
}

func (r *CrdbBackubJobReco) CreateObj() (ctrl.Result, error) {
	if r.backupJob.Status.JobId != 0 {
		// Skip, job already exists. We we're only reloading the status
		return ctrl.Result{}, nil
	}

	storageInfo, dbInfo, err := r.GetBackupTargetFull(r.backupJob.Spec.BackupTarget)
	if err != nil {
		return r.buildRetryResult(), nil
	}
	backupTarget, err := r.GetBackupTarget(r.backupJob.Spec.BackupTarget)
	if err != nil {
		return r.buildRetryResult(), nil
	}

	pgConn, err := r.getPostgresConnectionFromDbInfo(dbInfo)
	if err != nil {
		r.LogError(err, fmt.Sprint(err))
		return r.buildRetryResult(), nil
	}

	bucketInfo, err := storageInfo.GetBucketStorageInfo()
	if err != nil {
		r.LogError(err, fmt.Sprint(err))
		return r.buildRetryResult(), nil
	}

	bucketSecret := ""
	if len(bucketInfo.KeyName) > 0 {
		secret := &v1.Secret{}
		nsName := types.NamespacedName{
			Name:      bucketInfo.K8sSecret,
			Namespace: r.nsNm.Namespace,
		}
		err := r.client.Get(r.ctx, nsName, secret)
		if err != nil {
			r.LogError(err, fmt.Sprint(err))
			return r.buildRetryResult(), nil
		}

		byts, found := secret.Data[bucketInfo.K8sSecretKey]
		if !found {
			err = fmt.Errorf("Unabled to find key %s in secret %s", bucketInfo.K8sSecretKey, bucketInfo.K8sSecret)
			r.LogError(err, fmt.Sprint(err))
			return r.buildRetryResult(), nil
		}
		bucketSecret = string(byts)
	}

	job_id, err := pgConn.CreateBackupJob(
		backupTarget.Spec.DbName,
		bucketSecret,
		bucketInfo,
	)
	if err != nil {
		r.LogError(err, fmt.Sprint(err))
		return r.buildRetryResult(), nil
	}

	r.SetStatus(&r.backupJob, r.ctx, dboperatorv1alpha1.CockroachDBBackupJobStatus{
		JobId:    job_id,
		Created:  metav1.Now(),
		Started:  metav1.Unix(0, 0),
		Finished: metav1.Unix(0, 0),
	})

	// Return retry to that we can load the rest of the job status
	// we started it async so we don't expect any result just yet
	return r.buildRetryResult(), nil
}

func (r *CrdbBackubJobReco) RemoveObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Forgetting backupJob %s", r.backupJob.Name))
	return ctrl.Result{}, nil
}

func (r *CockroachDBBackupJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("cockroachDBBackupJob", req.NamespacedName)

	rr := CrdbBackubJobReco{
		Reco:         Reco{r.Client, ctx, r.Log, req.NamespacedName},
		StatusClient: r,
	}
	return rr.Reco.Reconcile((&rr))
}

// SetupWithManager sets up the controller with the Manager.
func (r *CockroachDBBackupJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.CockroachDBBackupJob{}).
		Complete(r)
}
