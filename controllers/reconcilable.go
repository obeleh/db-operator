package controllers

import (
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
	EnsureCorrect() (ctrl.Result, error)
	GetCR() client.Object
	CleanupConn()
}

type Reco struct {
	shared.K8sClient
}

const DB_OPERATOR_FINALIZER = "db-operator.kubemaster.com/finalizer"

func (rc *Reco) AddFinalizerToCr(cr client.Object) bool {
	if !controllerutil.ContainsFinalizer(cr, DB_OPERATOR_FINALIZER) {
		controllerutil.AddFinalizer(cr, DB_OPERATOR_FINALIZER)
		return true
	}
	return false
}

func (rc *Reco) EnsureFinalizer(cr client.Object) (ctrl.Result, error) {
	changed := rc.AddFinalizerToCr(cr)
	if changed {
		err := rc.Client.Update(rc.Ctx, cr)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (rc *Reco) RemoveFinalizer(cr client.Object) error {
	controllerutil.RemoveFinalizer(cr, DB_OPERATOR_FINALIZER)
	return rc.Client.Update(rc.Ctx, cr)
}

func (r *Reco) LogAndBackoffCreation(err error, obj client.Object) (ctrl.Result, error) {
	r.LogError(err, fmt.Sprint(err))
	return shared.GradualBackoffRetry(obj.GetCreationTimestamp().Time), nil
}

func (r *Reco) LogAndBackoffDeletion(err error, obj client.Object) (ctrl.Result, error) {
	r.LogError(err, fmt.Sprint(err))
	return shared.GradualBackoffRetry(obj.GetDeletionTimestamp().Time), nil
}

func (rc *Reco) Reconcile(rcl Reconcilable) (ctrl.Result, error) {
	res, err := rcl.LoadCR()
	if err != nil {
		if !shared.CannotFindError(err, rc.Log, "", rc.NsNm.Namespace, rc.NsNm.Name) {
			rc.LogError(err, fmt.Sprintf("Failed loading %s.%s", rc.NsNm.Namespace, rc.NsNm.Name))
		}
		return res, nil
	}
	if res.Requeue {
		return res, nil
	}
	defer rcl.CleanupConn()
	rc.Log.Info(fmt.Sprintf("Reconciling %s.%s ", rc.NsNm.Namespace, rc.NsNm.Name))
	res = ctrl.Result{}
	err = nil
	cr := rcl.GetCR()
	markedToBeDeleted := cr.GetDeletionTimestamp() != nil

	exists, err := rcl.LoadObj()
	if err != nil {
		if !shared.CannotFindError(err, rc.Log, "", rc.NsNm.Namespace, rc.NsNm.Name) {
			rc.LogError(err, fmt.Sprintf("failed loadObj for %s.%s", rc.NsNm.Namespace, rc.NsNm.Name))
		} else if markedToBeDeleted {
			// if it's a "cannot find error" and current obj is marked to be deleted
			// The parent resource has been removed. This resource probably doesn't exist anymore
			rc.RemoveFinalizer(cr)
			return ctrl.Result{}, nil
		}
		return shared.GradualBackoffRetry(cr.GetCreationTimestamp().Time), nil
	}
	if exists {
		if markedToBeDeleted {
			rc.Log.Info(fmt.Sprintf("%s is marked to be deleted", cr.GetName()))
			if controllerutil.ContainsFinalizer(cr, DB_OPERATOR_FINALIZER) {
				res, err = rcl.RemoveObj()
				// if reconciler asks for reque, don't remove Finalizer yet
				if err == nil && !res.Requeue {
					err = rc.RemoveFinalizer(cr)
				}
			}
		} else {
			res, err = rcl.EnsureCorrect()
			if err == nil && !res.Requeue {
				res, err = rc.EnsureFinalizer(cr)
				if err != nil {
					// should be able to retry quickly since only we couldn't add the finalizer
					res = shared.RetryAfter(3)
					err = nil
				}
			}
		}
	} else {
		if markedToBeDeleted {
			err = rc.RemoveFinalizer(cr)
		} else {
			res, err = rcl.CreateObj()
			if err == nil && !res.Requeue {
				res, err = rc.EnsureFinalizer(cr)
			}
		}
	}
	if err != nil && !shared.IsHandledErr(err) {
		rc.LogError(err, fmt.Sprintf("Unhandled error in %s", shared.GetTypeName(rcl)))
		if cr != nil {
			if markedToBeDeleted {
				res = shared.GradualBackoffRetry(cr.GetDeletionTimestamp().Time)
			} else {
				res = shared.GradualBackoffRetry(cr.GetCreationTimestamp().Time)
			}
		} else {
			res = shared.RetryAfter(30)
		}
	}
	return res, nil
}

func (r *Reco) GetBackupTarget(backupTarget string) (*dboperatorv1alpha1.BackupTarget, error) {
	r.Log.Info(fmt.Sprintf("loading backupTarget %s", backupTarget))
	backupTargetCr := &dboperatorv1alpha1.BackupTarget{}
	nsName := types.NamespacedName{
		Name:      backupTarget,
		Namespace: r.NsNm.Namespace,
	}
	err := r.Client.Get(r.Ctx, nsName, backupTargetCr)
	return backupTargetCr, err
}

func (r *Reco) GetRestoreTarget(restoreTarget string) (*dboperatorv1alpha1.RestoreTarget, error) {
	r.Log.Info(fmt.Sprintf("loading backupTarget %s", restoreTarget))
	restoreTargetCr := &dboperatorv1alpha1.RestoreTarget{}
	nsName := types.NamespacedName{
		Name:      restoreTarget,
		Namespace: r.NsNm.Namespace,
	}
	err := r.Client.Get(r.Ctx, nsName, restoreTargetCr)
	return restoreTargetCr, err
}

func (r *Reco) GetDb(dbName string) (*dboperatorv1alpha1.Db, error) {
	r.Log.Info(fmt.Sprintf("loading db %s", dbName))
	db := &dboperatorv1alpha1.Db{}
	nsName := types.NamespacedName{
		Name:      dbName,
		Namespace: r.NsNm.Namespace,
	}
	err := r.Client.Get(r.Ctx, nsName, db)
	return db, err
}

func (r *Reco) EnsureScripts() error {
	r.Log.Info("Ensure scripts")
	cm := &v1.ConfigMap{}
	nsName := types.NamespacedName{
		Name:      shared.SCRIPTS_CONFIGMAP,
		Namespace: r.NsNm.Namespace,
	}
	err := r.Client.Get(r.Ctx, nsName, cm)
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
			r.Client.Delete(r.Ctx, cm)
			cm = &v1.ConfigMap{}
		}
	}

	cm.Data = shared.SCRIPTS_MAP
	cm.Name = nsName.Name
	cm.Namespace = nsName.Namespace

	r.Log.Info("Creating scripts cm")
	err = r.Client.Create(r.Ctx, cm)
	if err != nil && !shared.AlreadyExistsError(err, r.Log, cm.Kind, cm.Namespace, cm.Name) {
		r.LogError(err, "failed creating configmap with scripts")
	}
	return nil
}

func (r *Reco) GetS3Storage(storageLocation string) (dboperatorv1alpha1.S3Storage, error) {
	s3Storage := &dboperatorv1alpha1.S3Storage{}
	nsName := types.NamespacedName{
		Name:      storageLocation,
		Namespace: r.NsNm.Namespace,
	}

	err := r.Client.Get(r.Ctx, nsName, s3Storage)
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
			Namespace: r.NsNm.Namespace,
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
			Namespace: r.NsNm.Namespace,
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
	dbServer, err := GetDbServer(db.Spec.Server, r.Client, r.NsNm.Namespace)
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
		client.InNamespace(r.NsNm.Namespace),
		client.MatchingLabels{"controlledBy": "DbOperator"},
	}
	err = r.Client.List(r.Ctx, jobs, opts...)
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
		client.InNamespace(r.NsNm.Namespace),
		client.MatchingLabels{"controlledBy": "DbOperator"},
	}
	err = r.Client.List(r.Ctx, cronJobs, opts...)
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

	err := r.Client.Get(r.Ctx, secretName, secret)
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

	keys := []struct {
		specKey  *string
		credsKey **string
	}{
		{&dbServer.Spec.CaCertKey, &creds.CaCert},
		{&dbServer.Spec.TlsKeyKey, &creds.TlsKey},
		{&dbServer.Spec.TlsCrtKey, &creds.TlsCrt},
	}

	for _, key := range keys {
		if *key.specKey != "" {
			valueBytes, found := secret.Data[*key.specKey]
			if !found {
				return creds, fmt.Errorf("key '%s' not found in secret %s.%s", *key.specKey, dbServer.Namespace, dbServer.Spec.SecretName)
			}
			value := string(valueBytes)
			*key.credsKey = &value
		}
	}

	return creds, nil
}

func (r *Reco) GetCredentialsForUser(namespace, userName string) (*shared.Credentials, error) {
	user := dboperatorv1alpha1.User{}
	userNsm := types.NamespacedName{
		Name:      userName,
		Namespace: namespace,
	}
	err := r.Client.Get(r.Ctx, userNsm, &user)
	if err != nil {
		return nil, err
	}

	return GetUserCredentials(&user, r.Client, r.Ctx)
}

func (r *Reco) GetConnectInfo(dbServer *dboperatorv1alpha1.DbServer, databaseName *string) (*shared.DbServerConnectInfo, error) {
	credentials, err := r.GetCredentials(dbServer)
	if err != nil {
		return nil, err
	}
	connectInfo := &shared.DbServerConnectInfo{
		Host:        dbServer.Spec.Address,
		Port:        dbServer.Spec.Port,
		Credentials: credentials,
	}

	if len(dbServer.Spec.Options) > 0 {
		connectInfo.Options = dbServer.Spec.Options
	}

	if databaseName != nil {
		connectInfo.Database = *databaseName
	}

	return connectInfo, nil
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

func (r *Reco) GetDbConnection(dbServer *dboperatorv1alpha1.DbServer, grantorNames []string, databaseName *string) (shared.DbServerConnectionInterface, error) {
	connectInfo, err := r.GetConnectInfo(dbServer, databaseName)
	if err != nil {
		return nil, err
	}

	userCredentials := map[string]*shared.Credentials{}
	for _, userName := range grantorNames {
		credentials, err := r.GetCredentialsForUser(r.NsNm.Namespace, userName)
		if err != nil {
			return nil, err
		}
		userCredentials[userName] = credentials
	}

	return dbservers.GetServerConnection(dbServer.Spec.ServerType, connectInfo, userCredentials)
}
