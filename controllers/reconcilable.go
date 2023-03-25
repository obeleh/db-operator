package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/dbservers"
	"github.com/obeleh/db-operator/shared"
	"go.uber.org/zap"
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
	Log    *zap.Logger
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
		if !shared.CannotFindError(err, rc.Log, "", rc.nsNm.Namespace, rc.nsNm.Name) {
			rc.LogError(err, fmt.Sprintf("Failed loading %s.%s", rc.nsNm.Namespace, rc.nsNm.Name))
		}
		// Not found
		return res, nil
	}

	rc.Log.Info(fmt.Sprintf("Reconciling %s.%s ", rc.nsNm.Namespace, rc.nsNm.Name))
	res = ctrl.Result{}
	err = nil
	cr := rcl.GetCR()
	markedToBeDeleted := cr.GetDeletionTimestamp() != nil

	exists, err := rcl.LoadObj()
	if err != nil {
		if shared.CannotFindError(err, rc.Log, "", rc.nsNm.Namespace, rc.nsNm.Name) {
			exists = false
		} else {
			return res, nil
		}
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
			} else if cr != nil {
				res = shared.GradualBackoffRetry(cr.GetCreationTimestamp().Time)
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
	if err != nil && !shared.IsHandledErr(err) {
		rc.LogError(err, fmt.Sprintf("Unhandled error in %s", shared.GetTypeName(rcl)))
		if cr != nil {
			res = shared.GradualBackoffRetry(cr.GetCreationTimestamp().Time)
		} else {
			res = shared.RetryAfter(30)
		}
	}
	rcl.CleanupConn()
	return res, nil
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
	if err != nil && !shared.AlreadyExistsError(err, r.Log, cm.Kind, cm.Namespace, cm.Name) {
		r.LogError(err, "failed creating configmap with scripts")
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

func (r *Reco) GetDbServerFromDbName(dbName string) (*dboperatorv1alpha1.Db, *dboperatorv1alpha1.DbServer, error) {
	db, err := r.GetDb(dbName)
	if err != nil {
		return nil, nil, err
	}
	dbServer, err := r.GetDbServer(db.Spec.Server)
	return db, dbServer, err
}

func (r *Reco) GetServerActionsFromDbName(dbName string) (shared.DbActions, error) {
	db, dbServer, err := r.GetDbServerFromDbName(dbName)
	if err != nil {
		return nil, err
	}
	return dbservers.GetServerActions(dbServer, db, dbServer.Spec.Options)
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
	r.Log.Error(message, zap.Error(err))
}

func (r *Reco) GetDbServerSecrets(dbServer *dboperatorv1alpha1.DbServer) (v1.Secret, shared.Credentials, error) {
	secretName := types.NamespacedName{
		Name:      dbServer.Spec.SecretName,
		Namespace: dbServer.Namespace,
	}
	secret := &v1.Secret{}

	creds := shared.Credentials{
		UserName:     dbServer.Spec.UserName,
		SourceSecret: &secretName,
	}

	err := r.client.Get(r.ctx, secretName, secret)
	if err != nil {
		err = fmt.Errorf("failed to get secret: %s %s", dbServer.Spec.SecretName, err)
	}

	return *secret, creds, err
}

func (r *Reco) GetCredentials(dbServer *dboperatorv1alpha1.DbServer) (shared.Credentials, error) {
	secret, creds, err := r.GetDbServerSecrets(dbServer)
	if err != nil {
		return creds, err
	}

	passwordBytes, found := secret.Data[shared.Nvl(dbServer.Spec.PasswordKey, "password")]
	if found {
		password := string(passwordBytes)
		creds.Password = &password
	}

	if dbServer.Spec.CaCertKey != "" {
		caCertBytes, found := secret.Data[dbServer.Spec.CaCertKey]
		if !found {
			return creds, fmt.Errorf("ca_cert_key '%s' not found in secret %s.%s", dbServer.Spec.CaCertKey, dbServer.Namespace, dbServer.Spec.SecretName)
		}
		caCert := string(caCertBytes)
		creds.CaCert = &caCert
	}
	if dbServer.Spec.TlsKeyKey != "" {
		tlsKeyBytes, found := secret.Data[dbServer.Spec.TlsKeyKey]
		if !found {
			return creds, fmt.Errorf("tls_key_key '%s' not found in secret %s.%s", dbServer.Spec.TlsKeyKey, dbServer.Namespace, dbServer.Spec.SecretName)
		}
		tlsKey := string(tlsKeyBytes)
		creds.TlsKey = &tlsKey
	}
	if dbServer.Spec.TlsCrtKey != "" {
		tlsCrtBytes, found := secret.Data[dbServer.Spec.TlsCrtKey]
		if !found {
			return creds, fmt.Errorf("tls_cert_key '%s' not found in secret %s.%s", dbServer.Spec.TlsCrtKey, dbServer.Namespace, dbServer.Spec.SecretName)
		}
		tlsCrt := string(tlsCrtBytes)
		creds.TlsCrt = &tlsCrt
	}
	return creds, nil
}

func (r *Reco) GetCredentialsForUser(namespace, userName string) (*shared.Credentials, error) {
	user := dboperatorv1alpha1.User{}
	userNsm := types.NamespacedName{
		Name:      userName,
		Namespace: namespace,
	}
	err := r.client.Get(r.ctx, userNsm, &user)
	if err != nil {
		return nil, err
	}

	return GetUserCredentials(&user, r.client, r.ctx)
}

func (r *Reco) GetConnectInfo(dbServer *dboperatorv1alpha1.DbServer) (*shared.DbServerConnectInfo, error) {
	credentials, err := r.GetCredentials(dbServer)
	if err != nil {
		return nil, err
	}
	return &shared.DbServerConnectInfo{
		Host:        dbServer.Spec.Address,
		Port:        dbServer.Spec.Port,
		Credentials: credentials,
	}, nil
}

func (r *Reco) GetStorageActions(storageType string, storageLocation string) (StorageActions, error) {
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

func (r *Reco) GetDbConnection(dbServer *dboperatorv1alpha1.DbServer, userNames []string) (shared.DbServerConnectionInterface, error) {
	connectInfo, err := r.GetConnectInfo(dbServer)
	if err != nil {
		return nil, err
	}

	userCredentials := map[string]*shared.Credentials{}
	if len(userNames) > 0 {
		for _, userName := range userNames {
			credentials, err := r.GetCredentialsForUser(r.nsNm.Namespace, userName)
			if err != nil {
				return nil, err
			}
			userCredentials[userName] = credentials
		}
	}

	return dbservers.GetServerConnection(dbServer.Spec.ServerType, connectInfo, userCredentials)
}
