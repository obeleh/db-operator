package controllers

import (
	"context"
	"fmt"
	"log"
	path "path/filepath"
	"regexp"

	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
	_ "github.com/lib/pq"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const SCRIPTS_CONFIGMAP string = "db-operator-scripts"

var VOLUME_MOUNTS = []v1.VolumeMount{
	{Name: SCRIPTS_VOLUME_NAME, MountPath: path.Join("/", SCRIPTS_VOLUME_NAME)},
	{Name: BACKUP_VOLUME_NAME, MountPath: path.Join("/", BACKUP_VOLUME_NAME)},
}

func GetVolumes() []v1.Volume {
	var defaultMode = new(int32)
	*defaultMode = 511 //  0777

	return []v1.Volume{
		{
			Name: SCRIPTS_VOLUME_NAME,
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: SCRIPTS_CONFIGMAP,
					},
					Items:       []v1.KeyToPath{},
					DefaultMode: defaultMode,
				},
			},
		},
		{
			Name: BACKUP_VOLUME_NAME,
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
	}
}

func GetUserPassword(dbUser *dboperatorv1alpha1.User, k8sClient client.Client, ctx context.Context) (*string, error) {
	secretName := types.NamespacedName{
		Name:      dbUser.Spec.SecretName,
		Namespace: dbUser.Namespace,
	}
	secret := &v1.Secret{}
	err := k8sClient.Get(ctx, secretName, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %s", dbUser.Spec.SecretName)
	}

	passBytes, ok := secret.Data[Nvl(dbUser.Spec.SecretKey, "password")]
	if !ok {
		return nil, fmt.Errorf("password key (%s) not found in secret", Nvl(dbUser.Spec.SecretKey, "password"))
	}

	password := string(passBytes)

	return &password, nil
}

func Nvl(val1 string, val2 string) string {
	if len(val1) == 0 {
		return val2
	} else {
		return val1
	}
}

func ReplaceNonAllowedChars(input string) string {
	reg, err := regexp.Compile("[^A-Za-z0-9]+")
	if err != nil {
		log.Fatal(err)
	}
	return reg.ReplaceAllString(input, "-")
}
