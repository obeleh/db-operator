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
	"fmt"
	"strings"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/shared"
)

// SchemaReconciler reconciles a Schema object
type SchemaReconciler struct {
	client.Client
	Log    *zap.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=schemas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=schemas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db-operator.kubemaster.com,resources=schemas/finalizers,verbs=update

type SchemaReco struct {
	Reco
	db      dboperatorv1alpha1.Db
	schema  dboperatorv1alpha1.Schema
	schemas map[string]shared.DbSideSchema
	conn    shared.DbServerConnectionInterface
}

func (s *SchemaReco) MarkedToBeDeleted() bool {
	return s.schema.GetDeletionTimestamp() != nil
}

func (s *SchemaReco) LoadCR() (ctrl.Result, error) {
	err := s.client.Get(s.ctx, s.nsNm, &s.schema)
	if err != nil {
		s.Log.Info(fmt.Sprintf("%T: %s does not exist, %s", s.schema, s.nsNm.Name, err))
		return ctrl.Result{}, err
	}

	err = s.client.Get(s.ctx, s.nsNm, &s.db)
	if err != nil {
		s.Log.Info(fmt.Sprintf("%T: %s does not exist, %s", s.db, s.nsNm.Name, err))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (s *SchemaReco) LoadObj() (bool, error) {
	var err error
	// First create conninfo without db name because we don't know whether it exists
	dbServer, err := s.GetDbServer(s.schema.Spec.Server)
	if err != nil {
		if !shared.CannotFindError(err, s.Log, "DbServer", s.nsNm.Namespace, s.nsNm.Name) {
			s.LogError(err, "failed getting DbServer")
			return false, err
		}
		return false, nil
	}

	s.conn, err = s.GetDbConnection(dbServer, &s.db)
	if err != nil {
		errStr := err.Error()
		if !strings.Contains(errStr, "failed getting password failed to get secret") {
			s.LogError(err, "failed building dbConnection")
		}
		return false, err
	}

	s.schemas, err = s.conn.GetSchemas()
	if err != nil {
		s.LogError(err, "failed getting DBs")
		return false, err
	}
	_, exists := s.schemas[s.schema.Spec.Name]
	return exists, nil
}

func (s *SchemaReco) CreateObj() (ctrl.Result, error) {
	s.Log.Info(fmt.Sprintf("Creating schema %s", s.schema.Spec.Name))
	var err error
	if s.conn == nil {
		message := "no database connection possible"
		err = fmt.Errorf(message)
		s.LogError(err, message)
		return ctrl.Result{}, err
	}
	err = s.conn.CreateSchema(s.schema.Spec.Name)
	if err != nil {
		s.LogError(err, fmt.Sprintf("failed to Create Schema: %s", s.schema.Spec.Name))
		return shared.GradualBackoffRetry(s.schema.GetCreationTimestamp().Time), nil
	}
	return ctrl.Result{}, nil
}

func (s *SchemaReco) RemoveObj() (ctrl.Result, error) {
	if s.schema.Spec.DropOnDeletion {
		s.Log.Info(fmt.Sprintf("dropping schema %s.%s", s.schema.Spec.DbName, s.schema.Name))
		err := s.conn.DropSchema(s.schema.Name)
		if err != nil {
			s.LogError(err, fmt.Sprintf("failed to drop schema %s\n%s", s.schema.Spec.DbName, err))
			return shared.GradualBackoffRetry(s.schema.GetCreationTimestamp().Time), err
		}
		s.Log.Info(fmt.Sprintf("finalized schema %s.%s", s.schema.Spec.DbName, s.schema.Spec.Name))
	} else {
		s.Log.Info(fmt.Sprintf("did not drop db %s.%s", s.schema.Spec.DbName, s.schema.Spec.Name))
	}
	return ctrl.Result{}, nil
}

func (s *SchemaReco) GetCR() client.Object {
	return &s.schema
}

func (s *SchemaReco) NotifyChanges() {
}

func (s *SchemaReco) EnsureCorrect() (bool, error) {
	return false, nil
}

func (s *SchemaReco) CleanupConn() {
	if s.conn != nil {
		s.conn.Close()
	}
}

func (s *SchemaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := s.Log.With(zap.String("Namespace", req.Namespace)).With(zap.String("Name", req.Name))
	sr := DbReco{}
	sr.Reco = Reco{s.Client, ctx, log, req.NamespacedName}
	return sr.Reco.Reconcile(&sr)
}

// SetupWithManager sets up the controller with the Manager.
func (s *SchemaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dboperatorv1alpha1.Schema{}).
		Complete(s)
}
