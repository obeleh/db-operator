/*
Copyright 2021.

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

	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/shared"
)

// RestoreCronJobReconciler reconciles a RestoreCronJob object
type RestoreCronJobReconciler struct {
	client.Client
	Log    *zap.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=restorecronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=restorecronjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=restorecronjobs/finalizers,verbs=update

type RestoreCronJobReco struct {
	Reco
	restoreCronJob          dboperatorv1alpha1.RestoreCronJob
	restoreCronJobs         map[string]batchv1.CronJob
	lazyRestoreTargetHelper *LazyRestoreTargetHelper
}

func (r *RestoreCronJobReco) LoadObj() (bool, error) {
	r.Log.Info(fmt.Sprintf("loading restoreCronJob %s", r.restoreCronJob.Name))
	var err error
	r.restoreCronJobs, err = r.GetCronJobMap()
	if err != nil {
		if !shared.CannotFindError(err, r.Log, "RestoreCronJob", r.NsNm.Namespace, r.NsNm.Name) {
			r.LogError(err, "failed getting RestoreCronJob")
			return false, err
		}
		return false, nil
	}

	_, exists := r.restoreCronJobs[r.restoreCronJob.Name]
	r.Log.Info(fmt.Sprintf("restoreCronJob %s exists: %t", r.restoreCronJob.Name, exists))
	return exists, nil
}

func (r *RestoreCronJobReco) CreateObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("creating restoreCronJob %s", r.restoreCronJob.Name))

	err := r.EnsureScripts()
	if err != nil {
		return r.LogAndBackoffCreation(err, r.GetCR())
	}

	storageInfo, actions, err := r.lazyRestoreTargetHelper.GetStorageInfoAndActions()
	restoreContainer := actions.BuildRestoreContainer()
	downloadContainer := storageInfo.BuildDownloadContainer(r.restoreCronJob.Spec.FixedFileName)
	cronJob := r.BuildCronJob(
		[]v1.Container{downloadContainer},
		restoreContainer,
		r.restoreCronJob.Name,
		r.restoreCronJob.Spec.Interval,
		r.restoreCronJob.Spec.Suspend,
		r.restoreCronJob.Spec.ServiceAccount,
	)

	err = r.Client.Create(r.Ctx, &cronJob)
	if err != nil && !shared.AlreadyExistsError(err, r.Log, cronJob.Kind, cronJob.Namespace, cronJob.Name) {
		r.LogError(err, "Failed to create restore cronjob")
	}
	return ctrl.Result{}, nil
}

func (r *RestoreCronJobReco) RemoveObj() (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Removing restoreCronJob %s", r.restoreCronJob.Name))
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.restoreCronJob.Name,
			Namespace: r.NsNm.Namespace,
		},
	}
	err := r.Client.Delete(r.Ctx, cronJob)
	if err != nil {
		return r.LogAndBackoffDeletion(err, r.GetCR())
	}
	return ctrl.Result{}, nil
}

func (r *RestoreCronJobReco) LoadCR() (ctrl.Result, error) {
	err := r.Client.Get(r.Ctx, r.NsNm, &r.restoreCronJob)
	if err != nil {
		r.Log.Info(fmt.Sprintf("%T: %s does not exist", r.restoreCronJob, r.NsNm.Name))
		return r.LogAndBackoffCreation(err, r.GetCR())
	}
	r.lazyRestoreTargetHelper = NewLazyRestoreTargetHelper(&r.K8sClient, r.restoreCronJob.Spec.RestoreTarget)
	return ctrl.Result{}, nil
}

func (r *RestoreCronJobReco) GetCR() client.Object {
	return &r.restoreCronJob
}

func (r *RestoreCronJobReco) EnsureCorrect() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *RestoreCronJobReco) CleanupConn() {
	if r.lazyRestoreTargetHelper != nil {
		r.lazyRestoreTargetHelper.CleanupConn()
	}
}

func (r *RestoreCronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.With(zap.String("Namespace", req.Namespace)).With(zap.String("Name", req.Name))

	rr := RestoreCronJobReco{
		Reco: Reco{shared.K8sClient{Client: r.Client, Ctx: ctx, NsNm: req.NamespacedName, Log: log}},
	}
	return rr.Reco.Reconcile((&rr))
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreCronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.RestoreCronJob{}).
		Complete(r)
}
