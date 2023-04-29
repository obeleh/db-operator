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

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/shared"
)

// CockroachDBBackupJobReconciler reconciles a CockroachDBBackupJob object
type CockroachDBBackupJobReconciler struct {
	client.Client
	Log    *zap.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=cockroachdbbackupjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=cockroachdbbackupjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=cockroachdbbackupjobs/finalizers,verbs=update

type CrdbBackubJobReco struct {
	Reco
	backupJob              dboperatorv1alpha1.CockroachDBBackupJob
	StatusClient           client.StatusClient
	lazyBackupTargetHelper *LazyBackupTargetHelper
}

func (r *CrdbBackubJobReco) LoadObj() (bool, error) {
	r.Log.Info(fmt.Sprintf("loading Cockroachdb backupJob %s", r.backupJob.Name))
	var err error
	if r.backupJob.Status.JobId == 0 {
		r.Log.Info(fmt.Sprintf("backupJob %s does not have a job_id, ignoring as this job maybe have been executed without the operator recording an id", r.backupJob.Name))
		return false, nil
	}

	pgConn, err := r.lazyBackupTargetHelper.GetPgConnection()
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
	jobStatus, err := jobMapToJobStatus(jobMap)
	if err != nil {
		return false, err
	}
	err = r.SetStatus(jobStatus)
	if err != nil {
		return false, err
	}

	r.Log.Info(fmt.Sprintf("backupJob %s exists with ID: %d", r.backupJob.Name, r.backupJob.Status.JobId))
	return true, nil
}

/* TODO: Can we remove this?
func (r *CrdbBackubJobReco) GetJobMap() (map[int64]dboperatorv1alpha1.CockroachDBBackupJobStatus, error) {
	jobsMap := make(map[int64]dboperatorv1alpha1.CockroachDBBackupJobStatus)

	pgConn, err := r.lazyBackupTargetHelper.GetPgConnection()
	if err != nil {
		return nil, err
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
		r.Log.Info(fmt.Sprintf("Found job %d", jobStatus.JobId))
		jobsMap[jobStatus.JobId] = jobStatus
	}
	return jobsMap, nil
}*/

func (r *CrdbBackubJobReco) SetStatus(newStatus dboperatorv1alpha1.CockroachDBBackupJobStatus) error {
	if !reflect.DeepEqual(r.backupJob.Status, newStatus) {
		r.backupJob.Status = newStatus
		err := r.StatusClient.Status().Update(r.Ctx, &r.backupJob)
		if err != nil {
			return err
		}
		// Add finalizer here because reco doesn't add finalizer to requeues
		_, err = r.EnsureFinalizer(&r.backupJob)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *CrdbBackubJobReco) LoadCR() (ctrl.Result, error) {
	err := r.Client.Get(r.Ctx, r.NsNm, &r.backupJob)
	if err != nil {
		r.Log.Info(fmt.Sprintf("%T: %s does not exist", r.backupJob, r.NsNm.Name))
		return r.LogAndBackoffCreation(err, r.GetCR())
	}

	r.lazyBackupTargetHelper = NewLazyBackupTargetHelper(&r.K8sClient, r.backupJob.Spec.BackupTarget)
	return ctrl.Result{}, nil
}

func (r *CrdbBackubJobReco) GetCR() client.Object {
	return &r.backupJob
}

func (r *CrdbBackubJobReco) BackupEnded() bool {
	return r.backupJob.Status.Status == "failed" || r.backupJob.Status.Status == "succeeded"
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (r *CrdbBackubJobReco) EnsureCorrect() (ctrl.Result, error) {
	if !r.BackupEnded() {
		return shared.GradualBackoffRetry(r.backupJob.GetCreationTimestamp().Time), nil
	}
	return ctrl.Result{}, nil
}

func (r *CrdbBackubJobReco) CleanupConn() {
	if r.lazyBackupTargetHelper != nil {
		r.lazyBackupTargetHelper.CleanupConn()
	}
}

func (r *CrdbBackubJobReco) CreateObj() (ctrl.Result, error) {
	if r.backupJob.Status.JobId != 0 {
		// Skip, job already exists. We we're only reloading the status
		return ctrl.Result{}, nil
	}

	pgConn, err := r.lazyBackupTargetHelper.GetPgConnection()
	if err != nil {
		return r.LogAndBackoffCreation(err, r.GetCR())
	}

	dbName, err := r.lazyBackupTargetHelper.GetDbName()
	if err != nil {
		return r.LogAndBackoffCreation(err, r.GetCR())
	}

	bucketStorageInfo, err := r.lazyBackupTargetHelper.GetBucketStorageInfo()
	if err != nil {
		return r.LogAndBackoffCreation(err, r.GetCR())
	}

	//dbName string, bucketSecret string, bucketStorageInfo shared.BucketStorageInfo) (int64, error) {
	job_id, err := pgConn.CreateBackupJob(
		dbName,
		bucketStorageInfo,
	)
	if err != nil {
		r.LogError(err, fmt.Sprint(err))
		return shared.GradualBackoffRetry(r.backupJob.GetCreationTimestamp().Time), nil
	}

	println("Jobid", job_id)
	r.SetStatus(dboperatorv1alpha1.CockroachDBBackupJobStatus{
		JobId:    job_id,
		Created:  metav1.Now(),
		Started:  metav1.Unix(0, 0),
		Finished: metav1.Unix(0, 0),
	})

	// Return retry to that we can load the rest of the job status
	// we started it async so we don't expect any result just yet
	return shared.GradualBackoffRetry(r.backupJob.GetCreationTimestamp().Time), nil
}

func (r *CrdbBackubJobReco) RemoveObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Forgetting backupJob %s", r.backupJob.Name))
	return ctrl.Result{}, nil
}

func (r *CockroachDBBackupJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.With(zap.String("Namespace", req.Namespace)).With(zap.String("Name", req.Name))

	reco := Reco{shared.K8sClient{Client: r.Client, Ctx: ctx, NsNm: req.NamespacedName, Log: log}}
	rr := CrdbBackubJobReco{
		Reco:         reco,
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

func jobMapToJobStatus(jobMap map[string]interface{}) (dboperatorv1alpha1.CockroachDBBackupJobStatus, error) {
	createdT, found := jobMap["created"]
	var created metav1.Time
	if found && createdT != nil {
		created = metav1.NewTime(createdT.(time.Time))
	} else {
		created = metav1.Unix(0, 0)
	}

	startedT, found := jobMap["started"]
	var started metav1.Time
	if found && startedT != nil {
		started = metav1.NewTime(startedT.(time.Time))
	} else {
		started = metav1.Unix(0, 0)
	}

	finishedT, found := jobMap["finished"]
	var finished metav1.Time
	if found && finishedT != nil {
		finished = metav1.NewTime(finishedT.(time.Time))
	} else {
		finished = metav1.Unix(0, 0)
	}

	return dboperatorv1alpha1.CockroachDBBackupJobStatus{
		JobId:       jobMap["job_id"].(int64),
		Status:      jobMap["status"].(string),
		Description: jobMap["description"].(string),
		Created:     created,
		Started:     started,
		Finished:    finished,
		Error:       jobMap["error"].(string),
	}, nil
}
