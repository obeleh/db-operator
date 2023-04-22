package shared

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type K8sClient struct {
	Client client.Client
	Ctx    context.Context
	NsNm   types.NamespacedName
	Log    *zap.Logger
}
