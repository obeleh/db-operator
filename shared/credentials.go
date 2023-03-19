package shared

import "k8s.io/apimachinery/pkg/types"

type Credentials struct {
	UserName     string
	Password     *string
	CaCert       *string
	TlsCrt       *string
	TlsKey       *string
	SourceSecret *types.NamespacedName
}
