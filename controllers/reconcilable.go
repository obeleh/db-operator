package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta "k8s.io/api/batch/v1beta1"
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
	EnsureCorrect() (ctrl.Result, error)
	GetCR() client.Object
	CleanupConn()
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
				if err == nil {
					controllerutil.RemoveFinalizer(cr, DB_OPERATOR_FINALIZER)
					err = rc.client.Update(rc.ctx, cr)
				}
			}
		} else {
			res, err = rcl.EnsureCorrect()
			if err != nil {
				res, err = rc.EnsureFinalizer(cr)
			}
		}
	} else if !markedToBeDeleted {
		res, err = rcl.CreateObj()
		if err == nil {
			res, err = rc.EnsureFinalizer(cr)
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

func (r *Reco) GetS3Storage(storageLocation string) (dboperatorv1alpha1.S3Storage, error) {
	s3Storage := &dboperatorv1alpha1.S3Storage{}
	nsName := types.NamespacedName{
		Name:      storageLocation,
		Namespace: r.nsNm.Namespace,
	}

	err := r.client.Get(r.ctx, nsName, s3Storage)
	return *s3Storage, err
}

func (r *Reco) BuildJob(initContainers []v1.Container, container v1.Container, jobName string) batchv1.Job {
	podSpec := v1.PodSpec{
		InitContainers: initContainers,
		Containers: []v1.Container{
			container,
		},
		RestartPolicy: v1.RestartPolicyNever,
		Volumes:       GetVolumes(),
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

func (r *Reco) BuildCronJob(initContainers []v1.Container, container v1.Container, jobName string, schedule string) batchv1beta.CronJob {
	podSpec := v1.PodSpec{
		InitContainers: initContainers,
		Containers: []v1.Container{
			container,
		},
		RestartPolicy: v1.RestartPolicyNever,
		Volumes:       GetVolumes(),
	}

	return batchv1beta.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: r.nsNm.Namespace,
			Labels: map[string]string{
				"controlledBy": "DbOperator",
			},
		},
		Spec: batchv1beta.CronJobSpec{
			JobTemplate: batchv1beta.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: v1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			},
			Schedule: schedule,
		},
	}
}

func (r *Reco) GetBackupTargetFull(backupTargetName string) (*dboperatorv1alpha1.BackupTarget, *dboperatorv1alpha1.Db, *dboperatorv1alpha1.DbServer, error) {
	backupTarget, err := r.GetBackupTarget(backupTargetName)
	if err != nil {
		return nil, nil, nil, err
	}
	db, dbServer, err := r.GetDbFull(backupTarget.Spec.DbName)
	if err != nil {
		return nil, nil, nil, err
	}

	return backupTarget, db, dbServer, err
}

func (r *Reco) GetRestoreTargetFull(restoreTargetName string) (*dboperatorv1alpha1.RestoreTarget, *dboperatorv1alpha1.Db, *dboperatorv1alpha1.DbServer, error) {
	restoreTarget, err := r.GetRestoreTarget(restoreTargetName)
	if err != nil {
		return nil, nil, nil, err
	}
	db, dbServer, err := r.GetDbFull(restoreTarget.Spec.DbName)
	if err != nil {
		return nil, nil, nil, err
	}

	return restoreTarget, db, dbServer, err
}

func (r *Reco) GetDbFull(dbName string) (*dboperatorv1alpha1.Db, *dboperatorv1alpha1.DbServer, error) {
	db, err := r.GetDb(dbName)
	if err != nil {
		return nil, nil, err
	}
	dbServer, err := r.GetDbServer(db.Spec.Server)
	if err != nil {
		return nil, nil, err
	}
	err = r.EnsureScripts()
	if err != nil {
		return nil, nil, err
	}

	return db, dbServer, err
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
		r.Log.Error(err, "failed listing Jobs")
		return nil, err
	}
	jobsMap := make(map[string]batchv1.Job)
	for _, job := range jobs.Items {
		r.Log.Info(fmt.Sprintf("Found job %s", job.Name))
		jobsMap[job.Name] = job
	}
	return jobsMap, nil
}

func (r *Reco) GetCronJobMap() (map[string]batchv1beta.CronJob, error) {
	var err error
	cronJobs := &batchv1beta.CronJobList{}
	opts := []client.ListOption{
		client.InNamespace(r.nsNm.Namespace),
		client.MatchingLabels{"controlledBy": "DbOperator"},
	}
	err = r.client.List(r.ctx, cronJobs, opts...)
	if err != nil {
		r.Log.Error(err, "failed listing CronJobs")
		return nil, err
	}
	cronJobsMap := make(map[string]batchv1beta.CronJob)
	for _, cronJob := range cronJobs.Items {
		r.Log.Info(fmt.Sprintf("Found cronJob %s", cronJob.Name))
		cronJobsMap[cronJob.Name] = cronJob
	}
	return cronJobsMap, nil
}

func GetDbConnection(dbServer *dboperatorv1alpha1.DbServer, password string, database *string) (DbServerConnectionInterface, error) {
	if strings.ToLower(dbServer.Spec.ServerType) == "postgres" {
		var dbName string
		if database == nil {
			dbName = "postgres"
		} else {
			dbName = *database
		}
		conn := &PostgresConnection{
			DbServerConnection: DbServerConnection{
				DbServerConnectInfo: DbServerConnectInfo{
					Host:     dbServer.Spec.Address,
					Port:     dbServer.Spec.Port,
					UserName: dbServer.Spec.UserName,
					Password: password,
					Database: dbName,
				},
				Driver: "postgres",
			},
		}
		conn.DbServerConnectionInterface = conn
		return conn, nil
	} else if strings.ToLower(dbServer.Spec.ServerType) == "mysql" {
		var dbName string
		if database == nil {
			dbName = ""
		} else {
			dbName = *database
		}
		conn := &MySqlConnection{
			DbServerConnection: DbServerConnection{
				DbServerConnectInfo: DbServerConnectInfo{
					Host:     dbServer.Spec.Address,
					Port:     dbServer.Spec.Port,
					UserName: dbServer.Spec.UserName,
					Password: password,
					Database: dbName,
				},
				Driver: "mysql",
			},
		}
		conn.DbServerConnectionInterface = conn
		return conn, nil
	} else {
		return nil, fmt.Errorf("Expected either mysql or postgres server")
	}
}

func (r *Reco) GetDbConnection(dbServer *dboperatorv1alpha1.DbServer, database *string) (DbServerConnectionInterface, error) {
	secretName := types.NamespacedName{
		Name:      dbServer.Spec.SecretName,
		Namespace: dbServer.Namespace,
	}
	secret := &v1.Secret{}

	err := r.client.Get(r.ctx, secretName, secret)
	if err != nil {
		return nil, fmt.Errorf("Failed to get secret: %s", dbServer.Spec.SecretName)
	}

	password := string(secret.Data[Nvl(dbServer.Spec.SecretKey, "password")])

	return GetDbConnection(dbServer, password, database)
}

func (r *Reco) LogError(message string, err error) {
	r.Log.Error(err, fmt.Sprintf(message+" Error: %s", err))
}
