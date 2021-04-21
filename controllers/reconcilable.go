package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
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

func (rc *Reco) EnsureFinalizer(cr client.Object) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(cr, userFinalizer) {
		controllerutil.AddFinalizer(cr, userFinalizer)
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
	if !exists {
		res, err = rcl.CreateObj()
		if err == nil {
			res, err = rc.EnsureFinalizer(cr)
		}
	} else {
		if markedToBeDeleted {
			rc.Log.Info(fmt.Sprintf("%s is marked to be deleted", cr.GetName()))
			if controllerutil.ContainsFinalizer(cr, userFinalizer) {
				res, err = rcl.RemoveObj()
				if err == nil {
					controllerutil.RemoveFinalizer(cr, userFinalizer)
					err = rc.client.Update(rc.ctx, cr)
				}
			}
		} else {
			res, err = rcl.EnsureCorrect()
			if err != nil {
				res, err = rc.EnsureFinalizer(cr)
			}
		}
	}
	rcl.CleanupConn()
	return res, err
}
