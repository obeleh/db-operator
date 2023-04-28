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
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/dbservers/postgres"
	"github.com/obeleh/db-operator/shared"
)

// CockroachDBBackupCronJobReconciler reconciles a CockroachDBBackupCronJob object
type CockroachDBBackupCronJobReconciler struct {
	client.Client
	Log    *zap.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=cockroachdbbackupcronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=cockroachdbbackupcronjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=cockroachdbbackupcronjobs/finalizers,verbs=update

type CockroachDBBackupCronJobReco struct {
	Reco
	backupCronJob          dboperatorv1alpha1.CockroachDBBackupCronJob
	StatusClient           client.StatusClient
	lazyBackupTargetHelper *LazyBackupTargetHelper
}

func (r *CockroachDBBackupCronJobReco) MarkedToBeDeleted() bool {
	return r.backupCronJob.GetDeletionTimestamp() != nil
}

func (r *CockroachDBBackupCronJobReco) LoadObj() (bool, error) {
	r.Log.Info(fmt.Sprintf("loading Cockroachdb backupCronJob %s", r.backupCronJob.Name))
	var err error
	if r.backupCronJob.Status.ScheduleId == 0 {
		r.Log.Info(fmt.Sprintf("backupCronJob %s does not have a schedule_id, ignoring as this job maybe have been executed without the operator recording an id", r.backupCronJob.Name))
		return false, nil
	}

	pgConn, err := r.lazyBackupTargetHelper.GetPgConnection()
	if err != nil {
		return false, err
	}

	err = r.UpdateStatus(pgConn, r.backupCronJob.Status.ScheduleId)
	if err != nil {
		return false, err
	}

	r.Log.Info(fmt.Sprintf("backupCronJob %s exists with ID: %d", r.backupCronJob.Name, r.backupCronJob.Status.ScheduleId))
	return true, nil
}

func (r *CockroachDBBackupCronJobReco) UpdateStatus(pgConn *postgres.PostgresConnection, scheduleId int64) error {
	scheduleMap, err := pgConn.GetBackupScheduleById(scheduleId)
	if err != nil {
		return err
	}
	scheduleStatus, err := scheduleMapToJobStatus(scheduleMap)
	if err != nil {
		return err
	}
	return r.SetStatus(scheduleStatus)
}

func (r *CockroachDBBackupCronJobReco) SetStatus(newStatus dboperatorv1alpha1.CockroachDBBackupCronJobStatus) error {
	if !reflect.DeepEqual(r.backupCronJob.Status, newStatus) {
		r.backupCronJob.Status = newStatus
		err := r.StatusClient.Status().Update(context.Background(), &r.backupCronJob)
		if err != nil {
			message := fmt.Sprintf("failed patching status %s", err)
			r.Log.Info(message)
			return fmt.Errorf(message)
		}
	}
	return nil
}

func (r *CockroachDBBackupCronJobReco) LoadCR() (ctrl.Result, error) {
	err := r.Client.Get(r.Ctx, r.NsNm, &r.backupCronJob)
	if err != nil {
		r.Log.Info(fmt.Sprintf("%T: %s does not exist", r.backupCronJob, r.NsNm.Name))
		return r.LogAndBackoffCreation(err, r.GetCR())
	}
	r.lazyBackupTargetHelper = NewLazyBackupTargetHelper(&r.K8sClient, r.backupCronJob.Spec.BackupTarget)
	return ctrl.Result{}, nil
}

func (r *CockroachDBBackupCronJobReco) GetCR() client.Object {
	return &r.backupCronJob
}

func (r *CockroachDBBackupCronJobReco) EnsureCorrect() (ctrl.Result, error) {
	pgConn, err := r.lazyBackupTargetHelper.GetPgConnection()
	if err != nil {
		return r.LogAndBackoffCreation(err, r.GetCR())
	}
	storageInfo, err := r.lazyBackupTargetHelper.GetBucketStorageInfo()
	if err != nil {
		return r.LogAndBackoffCreation(err, r.GetCR())
	}
	dbName, err := r.lazyBackupTargetHelper.GetDbName()
	if err != nil {
		return r.LogAndBackoffCreation(err, r.GetCR())
	}

	// give redacted statment for comparison
	statement, err := pgConn.ConstructBackupJobStatement(storageInfo, dbName, "", true)
	if err != nil {
		return r.LogAndBackoffCreation(err, r.GetCR())
	}

	if statement != (*r.backupCronJob.Status.Command + ";") {
		err = pgConn.DropBackupSchedule(r.backupCronJob.Status.ScheduleId)
		if err != nil {
			return r.LogAndBackoffCreation(err, r.GetCR())
		}
		r.backupCronJob.Status.ScheduleId = 0
		_, err = r.CreateObj()
		if err != nil {
			return r.LogAndBackoffCreation(err, r.GetCR())
		}
	}

	scheduleMap, err := pgConn.GetBackupScheduleById(r.backupCronJob.Status.ScheduleId)
	if err != nil {
		return r.LogAndBackoffCreation(err, r.GetCR())
	}
	scheduleStatus, err := scheduleMapToJobStatus(scheduleMap)
	if err != nil {
		return r.LogAndBackoffCreation(err, r.GetCR())
	}

	if r.backupCronJob.Spec.Suspend {
		if scheduleStatus.ScheduleStatus != "PAUSED" {
			err = pgConn.PauseSchedule(r.backupCronJob.Status.ScheduleId)
		}
	} else {
		// if state contains something, probably an error, let's not overrule that
		if scheduleStatus.ScheduleStatus != "ACTIVE" && scheduleStatus.State == nil {
			err = pgConn.ResumeSchedule(r.backupCronJob.Status.ScheduleId)
		}
	}

	return ctrl.Result{}, nil
}

func (r *CockroachDBBackupCronJobReco) CleanupConn() {
	if r.lazyBackupTargetHelper != nil {
		r.lazyBackupTargetHelper.CleanupConn()
	}
}

func (r *CockroachDBBackupCronJobReco) CreateObj() (ctrl.Result, error) {
	if r.backupCronJob.Status.ScheduleId != 0 {
		// Skip, job already exists. We we're only reloading the status
		return ctrl.Result{}, nil
	}

	if r.backupCronJob.Spec.Suspend {
		err := fmt.Errorf("Unablable to create suspended backups, please enable suspended after first creation")
		return r.LogAndBackoffCreation(err, r.GetCR())
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

	runBackup := true
	arrayMap, err := pgConn.CreateBackupSchedule(
		dbName,
		bucketStorageInfo,
		r.backupCronJob.Name,
		r.backupCronJob.Spec.Interval,
		runBackup,
		r.backupCronJob.Spec.IgnoreExistingBackups,
	)
	if err != nil {
		return r.LogAndBackoffCreation(err, r.GetCR())
	}

	var scheduleMap map[string]interface{}
	if len(arrayMap) == 0 { // schedule existed
		scheduleMap, err = pgConn.GetBackupScheduleByLabel(r.backupCronJob.Name)
		if err != nil {
			return r.LogAndBackoffCreation(err, r.GetCR())
		}
	} else {
		scheduleMap = arrayMap[0]
	}
	scheduleStatus, err := scheduleMapToJobStatus(scheduleMap)
	if err != nil {
		return r.LogAndBackoffCreation(err, r.GetCR())
	}

	// Sleep a bit so that we increase the chance of getting the latest result.
	time.Sleep(3 * time.Second)

	err = r.UpdateStatus(pgConn, scheduleStatus.ScheduleId)
	if err != nil {
		return r.LogAndBackoffCreation(err, r.GetCR())
	}
	return ctrl.Result{}, nil
}

func (r *CockroachDBBackupCronJobReco) RemoveObj() (ctrl.Result, error) {
	if r.backupCronJob.Status.ScheduleId != 0 && r.backupCronJob.Spec.DropOnDeletion {
		pgConn, err := r.lazyBackupTargetHelper.GetPgConnection()
		if err != nil {
			return r.LogAndBackoffDeletion(err, r.GetCR())
		}
		err = pgConn.DropBackupSchedule(r.backupCronJob.Status.ScheduleId)
		if err != nil {
			return r.LogAndBackoffDeletion(err, r.GetCR())
		}
	}
	return ctrl.Result{}, nil
}

func (r *CockroachDBBackupCronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.With(zap.String("Namespace", req.Namespace)).With(zap.String("Name", req.Name))

	reco := Reco{shared.K8sClient{Client: r.Client, Ctx: ctx, NsNm: req.NamespacedName, Log: log}}
	rr := CockroachDBBackupCronJobReco{
		Reco:         reco,
		StatusClient: r,
	}
	return rr.Reco.Reconcile((&rr))
}

// SetupWithManager sets up the controller with the Manager.
func (r *CockroachDBBackupCronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.CockroachDBBackupCronJob{}).
		Complete(r)
}

func scheduleMapToJobStatus(scheduleMap map[string]interface{}) (dboperatorv1alpha1.CockroachDBBackupCronJobStatus, error) {
	createdT, found := scheduleMap["created"]
	var created metav1.Time
	if found && createdT != nil {
		created = metav1.NewTime(createdT.(time.Time))
	} else {
		created = metav1.Time{}
	}

	var command *string
	commandInterface, found := scheduleMap["command"]
	if found {
		commandMap := make(map[string]interface{})
		commandBytes := commandInterface.([]byte)
		err := json.Unmarshal(commandBytes, &commandMap)
		if err != nil {
			return dboperatorv1alpha1.CockroachDBBackupCronJobStatus{}, err
		}
		commandValue, commandValueFound := commandMap["backup_statement"]
		if commandValueFound {
			strValue := commandValue.(string)
			command = &strValue
		}
	}
	var state *string
	stateVl, found := scheduleMap["state"]
	if found && stateVl != nil {
		stateStr := stateVl.(string)
		state = &stateStr
	}

	var schduleId int64
	idVl, found := scheduleMap["id"]
	if found && idVl != nil {
		schduleId = idVl.(int64)
	}

	var scheduleStatus string
	statusVl, found := scheduleMap["schedule_status"]
	if found && statusVl != nil {
		scheduleStatus = statusVl.(string)
	}

	return dboperatorv1alpha1.CockroachDBBackupCronJobStatus{
		ScheduleId:     schduleId,
		ScheduleStatus: scheduleStatus,
		State:          state,
		Command:        command,
		Created:        created,
	}, nil
}
