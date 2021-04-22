package controllers

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	machineryErrors "k8s.io/apimachinery/pkg/api/errors"
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

func (r *Reco) GetDbServer(db *dboperatorv1alpha1.Db) (*dboperatorv1alpha1.DbServer, error) {
	r.Log.Info(fmt.Sprintf("loading dbServer %s", db.Spec.Server))
	dbServer := &dboperatorv1alpha1.DbServer{}
	nsName := types.NamespacedName{
		Name:      db.Spec.Server,
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

func (r *Reco) GetS3Storage(backupTarget *dboperatorv1alpha1.BackupTarget) (dboperatorv1alpha1.S3Storage, error) {
	s3Storage := &dboperatorv1alpha1.S3Storage{}
	nsName := types.NamespacedName{
		Name:      backupTarget.Spec.StorageLocation,
		Namespace: r.nsNm.Namespace,
	}

	err := r.client.Get(r.ctx, nsName, s3Storage)
	return *s3Storage, err
}
