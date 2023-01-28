package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/dbservers"
	"github.com/obeleh/db-operator/shared"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	machineryErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Reconcilable interface {
	CreateObj() (ctrl.Result, error)
	RemoveObj() (ctrl.Result, error)
	LoadCR() (ctrl.Result, error)
	LoadObj() (bool, error)
	EnsureCorrect() (bool, error)
	GetCR() client.Object
	CleanupConn()
	NotifyChanges()
	MarkedToBeDeleted() bool
}

type Reco struct {
	client client.Client
	ctx    context.Context
	Log    logr.Logger
	nsNm   types.NamespacedName
}

const DB_OPERATOR_FINALIZER = "db-operator.kubemaster.com/finalizer"

func (rc *Reco) EnsureFinalizer(cr client.Object) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(cr, DB_OPERATOR_FINALIZER) {
		controllerutil.AddFinalizer(cr, DB_OPERATOR_FINALIZER)
		err := rc.client.Update(rc.ctx, cr)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (rc *Reco) Reconcile(rcl Reconcilable) (ctrl.Result, error) {
	res, err := rcl.LoadCR()
	if err != nil {
		// Not found
		return res, nil
	}

	res = ctrl.Result{}
	err = nil
	cr := rcl.GetCR()
	markedToBeDeleted := cr.GetDeletionTimestamp() != nil

	exists, err := rcl.LoadObj()
	if err != nil {
		return res, nil
	}
	if exists {
		if markedToBeDeleted {
			rc.Log.Info(fmt.Sprintf("%s is marked to be deleted", cr.GetName()))
			if controllerutil.ContainsFinalizer(cr, DB_OPERATOR_FINALIZER) {
				res, err = rcl.RemoveObj()
				// if reconciler asks for reque, don't remove Finalizer yet
				if err == nil && !res.Requeue {
					controllerutil.RemoveFinalizer(cr, DB_OPERATOR_FINALIZER)
					err = rc.client.Update(rc.ctx, cr)
				}
				rcl.NotifyChanges()
			}
		} else {
			var changes bool
			changes, err = rcl.EnsureCorrect()
			if err == nil {
				res, err = rc.EnsureFinalizer(cr)
			}
			if changes {
				rcl.NotifyChanges()
			}
		}
	} else {
		if markedToBeDeleted {
			controllerutil.RemoveFinalizer(cr, DB_OPERATOR_FINALIZER)
			err = rc.client.Update(rc.ctx, cr)
		} else {
			res, err = rcl.CreateObj()
			if err == nil {
				res, err = rc.EnsureFinalizer(cr)
				rcl.NotifyChanges()
			}
		}
	}
	rcl.CleanupConn()
	return res, err
}

func (r *Reco) GetBackupTarget(backupTarget string) (*dboperatorv1alpha1.BackupTarget, error) {
	r.Log.Info(fmt.Sprintf("loading backupTarget %s", backupTarget))
	backupTargetCr := &dboperatorv1alpha1.BackupTarget{}
	nsName := types.NamespacedName{
		Name:      backupTarget,
		Namespace: r.nsNm.Namespace,
	}
	err := r.client.Get(r.ctx, nsName, backupTargetCr)
	return backupTargetCr, err
}

func (r *Reco) GetRestoreTarget(restoreTarget string) (*dboperatorv1alpha1.RestoreTarget, error) {
	r.Log.Info(fmt.Sprintf("loading backupTarget %s", restoreTarget))
	restoreTargetCr := &dboperatorv1alpha1.RestoreTarget{}
	nsName := types.NamespacedName{
		Name:      restoreTarget,
		Namespace: r.nsNm.Namespace,
	}
	err := r.client.Get(r.ctx, nsName, restoreTargetCr)
	return restoreTargetCr, err
}

func (r *Reco) GetDb(dbName string) (*dboperatorv1alpha1.Db, error) {
	r.Log.Info(fmt.Sprintf("loading db %s", dbName))
	db := &dboperatorv1alpha1.Db{}
	nsName := types.NamespacedName{
		Name:      dbName,
		Namespace: r.nsNm.Namespace,
	}
	err := r.client.Get(r.ctx, nsName, db)
	return db, err
}

func (r *Reco) GetDbServer(dbServerName string) (*dboperatorv1alpha1.DbServer, error) {
	r.Log.Info(fmt.Sprintf("loading dbServer %s", dbServerName))
	dbServer := &dboperatorv1alpha1.DbServer{}
	nsName := types.NamespacedName{
		Name:      dbServerName,
		Namespace: r.nsNm.Namespace,
	}
	err := r.client.Get(r.ctx, nsName, dbServer)
	return dbServer, err
}

func (r *Reco) EnsureScripts() error {
	r.Log.Info("Ensure scripts")
	cm := &v1.ConfigMap{}
	nsName := types.NamespacedName{
		Name:      shared.SCRIPTS_CONFIGMAP,
		Namespace: r.nsNm.Namespace,
	}
	err := r.client.Get(r.ctx, nsName, cm)
	found := true

	if err != nil {
		if machineryErrors.IsNotFound(err) {
			found = false
		} else {
			r.LogError(err, "Unable to lookup scripts CM")
			return err
		}
	}

	if found {
		if reflect.DeepEqual(cm.Data, shared.SCRIPTS_MAP) {
			r.Log.Info("Scripts existed and were up to date")
			return nil
		} else {
			r.client.Delete(r.ctx, cm)
			cm = &v1.ConfigMap{}
		}
	}

	cm.Data = shared.SCRIPTS_MAP
	cm.Name = nsName.Name
	cm.Namespace = nsName.Namespace

	r.Log.Info("Creating scripts cm")
	err = r.client.Create(r.ctx, cm)
	if err != nil {
		r.LogError(err, "failed creating cm")
		return fmt.Errorf("failed creating configmap with scripts")
	}
	return nil
}

func (r *Reco) GetS3Storage(storageLocation string) (dboperatorv1alpha1.S3Storage, error) {
	s3Storage := &dboperatorv1alpha1.S3Storage{}
	nsName := types.NamespacedName{
		Name:      storageLocation,
		Namespace: r.nsNm.Namespace,
	}

	err := r.client.Get(r.ctx, nsName, s3Storage)
	return *s3Storage, err
}

func (r *Reco) BuildJob(initContainers []v1.Container, container v1.Container, jobName string, serviceAccount string) batchv1.Job {
	podSpec := v1.PodSpec{
		InitContainers: initContainers,
		Containers: []v1.Container{
			container,
		},
		RestartPolicy: v1.RestartPolicyNever,
		Volumes:       shared.GetVolumes(),
	}

	if serviceAccount != "" {
		podSpec.ServiceAccountName = serviceAccount
	}

	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: r.nsNm.Namespace,
			Labels: map[string]string{
				"controlledBy": "DbOperator",
			},
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: podSpec,
			},
		},
	}
}

func (r *Reco) BuildCronJob(initContainers []v1.Container, container v1.Container, jobName string, schedule string, suspend bool, serviceAccount string) batchv1.CronJob {
	podSpec := v1.PodSpec{
		InitContainers: initContainers,
		Containers: []v1.Container{
			container,
		},
		RestartPolicy: v1.RestartPolicyNever,
		Volumes:       shared.GetVolumes(),
	}

	if serviceAccount != "" {
		podSpec.ServiceAccountName = serviceAccount
	}

	return batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: r.nsNm.Namespace,
			Labels: map[string]string{
				"controlledBy": "DbOperator",
			},
		},
		Spec: batchv1.CronJobSpec{
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: v1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			},
			Schedule: schedule,
			Suspend:  &suspend,
		},
	}
}

func (r *Reco) GetBackupTargetFull(backupTargetName string) (StorageActions, shared.DbActions, error) {
	backupTarget, err := r.GetBackupTarget(backupTargetName)
	if err != nil {
		return nil, nil, err
	}
	dbInfo, err := r.GetDbInfo(backupTarget.Spec.DbName)
	if err != nil {
		return nil, nil, err
	}
	storageInfo, err := r.GetStorageInfo(backupTarget.Spec.StorageType, backupTarget.Spec.StorageLocation)
	if err != nil {
		return nil, nil, err
	}
	return storageInfo, dbInfo, err
}

func (r *Reco) GetRestoreTargetFull(restoreTargetName string) (StorageActions, shared.DbActions, error) {
	restoreTarget, err := r.GetRestoreTarget(restoreTargetName)
	if err != nil {
		return nil, nil, err
	}
	dbInfo, err := r.GetDbInfo(restoreTarget.Spec.DbName)
	if err != nil {
		return nil, nil, err
	}
	storageInfo, err := r.GetStorageInfo(restoreTarget.Spec.StorageType, restoreTarget.Spec.StorageLocation)
	if err != nil {
		return nil, nil, err
	}
	return storageInfo, dbInfo, err
}

func (r *Reco) GetDbInfo(dbName string) (shared.DbActions, error) {
	db, err := r.GetDb(dbName)
	if err != nil {
		return nil, err
	}
	dbServer, err := r.GetDbServer(db.Spec.Server)
	if err != nil {
		return nil, err
	}
	return r.GetDbInfo2(dbServer, db)
}

func (r *Reco) GetDbInfo2(dbServer *dboperatorv1alpha1.DbServer, db *dboperatorv1alpha1.Db) (shared.DbActions, error) {
	password, err := r.GetPassword(dbServer)
	if err != nil {
		return nil, fmt.Errorf("failed getting password %s", err)
	}

	return dbservers.GetServerActions(dbServer.Spec.ServerType, dbServer, db, *password, dbServer.Spec.Options)
}

func (r *Reco) GetJobMap() (map[string]batchv1.Job, error) {
	var err error
	jobs := &batchv1.JobList{}
	opts := []client.ListOption{
		client.InNamespace(r.nsNm.Namespace),
		client.MatchingLabels{"controlledBy": "DbOperator"},
	}
	err = r.client.List(r.ctx, jobs, opts...)
	if err != nil {
		r.LogError(err, "failed listing Jobs")
		return nil, err
	}
	jobsMap := make(map[string]batchv1.Job)
	for _, job := range jobs.Items {
		r.Log.Info(fmt.Sprintf("Found job %s", job.Name))
		jobsMap[job.Name] = job
	}
	return jobsMap, nil
}

func (r *Reco) GetCronJobMap() (map[string]batchv1.CronJob, error) {
	var err error
	cronJobs := &batchv1.CronJobList{}
	opts := []client.ListOption{
		client.InNamespace(r.nsNm.Namespace),
		client.MatchingLabels{"controlledBy": "DbOperator"},
	}
	err = r.client.List(r.ctx, cronJobs, opts...)
	if err != nil {
		r.LogError(err, "failed listing CronJobs")
		return nil, err
	}
	cronJobsMap := make(map[string]batchv1.CronJob)
	for _, cronJob := range cronJobs.Items {
		r.Log.Info(fmt.Sprintf("Found cronJob %s", cronJob.Name))
		cronJobsMap[cronJob.Name] = cronJob
	}
	return cronJobsMap, nil
}

func (r *Reco) LogError(err error, message string) {
	r.Log.Error(err, fmt.Sprintf("%s Error: %s", message, err))
}

func (r *Reco) GetPassword(dbServer *dboperatorv1alpha1.DbServer) (*string, error) {
	secretName := types.NamespacedName{
		Name:      dbServer.Spec.SecretName,
		Namespace: dbServer.Namespace,
	}
	secret := &v1.Secret{}

	err := r.client.Get(r.ctx, secretName, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %s %s", dbServer.Spec.SecretName, err)
	}

	password := string(secret.Data[shared.Nvl(dbServer.Spec.SecretKey, "password")])
	return &password, nil
}

func (r *Reco) GetDbConnection(dbServer *dboperatorv1alpha1.DbServer, db *dboperatorv1alpha1.Db) (shared.DbServerConnectionInterface, error) {
	dbInfo, err := r.GetDbInfo2(dbServer, db)
	if err != nil {
		r.LogError(err, "failed getting dbInfo")
		return nil, err
	}

	return dbInfo.GetDbConnection()
}

func (r *Reco) GetStorageInfo(storageType string, storageLocation string) (StorageActions, error) {
	if strings.ToLower(storageType) == "s3" {
		s3, err := r.GetS3Storage(storageLocation)
		if err != nil {
			return nil, err
		}
		storage := &S3StorageInfo{
			S3Storage: s3,
		}
		return storage, nil
	} else {
		return nil, fmt.Errorf("unknown storage type %s", storageType)
	}
}
