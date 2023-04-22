package shared

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type BucketStorageInfo struct {
	StorageTypeName string
	BucketName      string
	Prefix          string
	Region          string
	KeyName         string
	K8sSecret       string
	K8sSecretKey    string
	AssumeRoleName  string
	Endpoint        string
	K8sClient       K8sClient
}

func (bi *BucketStorageInfo) GetBucketSecret() (string, error) {
	bucketSecret := ""
	if len(bi.KeyName) > 0 {
		secret := &v1.Secret{}
		nsName := types.NamespacedName{
			Name:      bi.K8sSecret,
			Namespace: bi.K8sClient.NsNm.Namespace,
		}
		err := bi.K8sClient.Client.Get(bi.K8sClient.Ctx, nsName, secret)
		if err != nil {
			return bucketSecret, err
		}

		byts, found := secret.Data[bi.K8sSecretKey]
		if !found {
			err = fmt.Errorf("Unabled to find key %s in secret %s", bi.K8sSecretKey, bi.K8sSecret)
			return bucketSecret, err
		}
		bucketSecret = string(byts)
	}

	return bucketSecret, nil
}
