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

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlZap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/controllers"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(dboperatorv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	flag.Parse()
	// Disabling stacktraces because they're not informative for the user
	stackTraceLevel, _ := zapcore.ParseLevel("dpanic")
	logger, err := zap.NewDevelopment(zap.AddStacktrace(stackTraceLevel))
	if err != nil {
		panic(fmt.Sprintf("Unable to construct logger %v", err))
	}

	ctrl.SetLogger(ctrlZap.New())

	err = godotenv.Load()
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("%v", err)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "5497fb29.kubemaster.com",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.DbCopyJobReconciler{
		Client: mgr.GetClient(),
		Log:    logger.With(zap.Namespace("DbCopyJobReconciler")),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DbCopyJob")
		os.Exit(1)
	}
	if err = (&controllers.BackupCronJobReconciler{
		Client: mgr.GetClient(),
		Log:    logger.With(zap.Namespace("BackupCronJobReconciler")),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BackupCronJob")
		os.Exit(1)
	}
	if err = (&controllers.BackupJobReconciler{
		Client: mgr.GetClient(),
		Log:    logger.With(zap.Namespace("BackupJobReconciler")),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BackupJob")
		os.Exit(1)
	}
	if err = (&controllers.BackupTargetReconciler{
		Client: mgr.GetClient(),
		Log:    logger.With(zap.Namespace("BackupTargetReconciler")),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BackupTarget")
		os.Exit(1)
	}
	if err = (&controllers.DbReconciler{
		Client: mgr.GetClient(),
		Log:    logger.With(zap.Namespace("DbReconciler")),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Db")
		os.Exit(1)
	}
	if err = (&controllers.DbCopyCronJobReconciler{
		Client: mgr.GetClient(),
		Log:    logger.With(zap.Namespace("DbCopyCronJobReconciler")),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DbCopyCronJob")
		os.Exit(1)
	}
	if err = (&controllers.DbServerReconciler{
		Client: mgr.GetClient(),
		Log:    logger.With(zap.Namespace("DbServerReconciler")),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DbServer")
		os.Exit(1)
	}
	if err = (&controllers.RestoreCronJobReconciler{
		Client: mgr.GetClient(),
		Log:    logger.With(zap.Namespace("RestoreCronJobReconciler")),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RestoreCronJob")
		os.Exit(1)
	}
	if err = (&controllers.RestoreJobReconciler{
		Client: mgr.GetClient(),
		Log:    logger.With(zap.Namespace("RestoreJobReconciler")),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RestoreJob")
		os.Exit(1)
	}
	if err = (&controllers.RestoreTargetReconciler{
		Client: mgr.GetClient(),
		Log:    logger.With(zap.Namespace("RestoreTargetReconciler")),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RestoreTarget")
		os.Exit(1)
	}
	if err = (&controllers.S3StorageReconciler{
		Client: mgr.GetClient(),
		Log:    logger.With(zap.Namespace("S3StorageReconciler")),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "S3Storage")
		os.Exit(1)
	}
	if err = (&controllers.UserReconciler{
		Client: mgr.GetClient(),
		Log:    logger.With(zap.Namespace("UserReconciler")),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "User")
		os.Exit(1)
	}
	if err = (&controllers.CockroachDBBackupJobReconciler{
		Client: mgr.GetClient(),
		Log:    logger.With(zap.Namespace("CockroachDBBackupJobReconciler")),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CockroachDBBackupJob")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
