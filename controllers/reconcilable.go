package controllers

import (
	"context"

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
	GetCR() client.Object
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
	if !exists {
		res, err = rcl.CreateObj()
		if err == nil {
			res, err = rc.EnsureFinalizer(cr)
		}
	} else {
		if markedToBeDeleted {
			if controllerutil.ContainsFinalizer(cr, userFinalizer) {
				res, err = rcl.RemoveObj()
				if err == nil {
					controllerutil.RemoveFinalizer(cr, userFinalizer)
					err = rc.client.Update(rc.ctx, cr)
				}
			}
		} else {
			res, err = rc.EnsureFinalizer(cr)
		}
	}
	return res, err
}
