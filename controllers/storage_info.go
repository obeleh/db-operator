package controllers

import (
	path "path/filepath"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/shared"
	v1 "k8s.io/api/core/v1"
)

type StorageActions interface {
	BuildContainer(script string, fixedFileName *string) v1.Container
	BuildUploadContainer(fixedFileName *string) v1.Container
	BuildDownloadContainer(fixedFileName *string) v1.Container
	GetBucketStorageInfo(shared.K8sClient) (shared.BucketStorageInfo, error)
}

type S3StorageInfo struct {
	S3Storage dboperatorv1alpha1.S3Storage
}

func (s *S3StorageInfo) GetBucketStorageInfo(k8sClient shared.K8sClient) (shared.BucketStorageInfo, error) {
	storageInfo := shared.BucketStorageInfo{
		StorageTypeName: "s3",
		BucketName:      s.S3Storage.Spec.BucketName,
		Prefix:          s.S3Storage.Spec.Prefix,
		Region:          s.S3Storage.Spec.Region,
		Endpoint:        s.S3Storage.Spec.Endpoint,
		K8sClient:       k8sClient,
	}
	if s.S3Storage.Spec.AccesKeyId != "" {
		storageInfo.KeyName = s.S3Storage.Spec.AccesKeyId
		storageInfo.K8sSecret = s.S3Storage.Spec.AccessKeyK8sSecret
		storageInfo.K8sSecretKey = shared.Nvl(s.S3Storage.Spec.AccessKeyK8sSecretKey, "SECRET_ACCESS_KEY")
	}
	return storageInfo, nil
}

func (s *S3StorageInfo) BuildContainer(script string, fixedFileName *string) v1.Container {
	envVars := []v1.EnvVar{
		{Name: "S3_BUCKET_NAME", Value: s.S3Storage.Spec.BucketName},
		{Name: "S3_PREFIX", Value: s.S3Storage.Spec.Prefix},
		{Name: "AWS_DEFAULT_REGION", Value: s.S3Storage.Spec.Region},
	}

	if s.S3Storage.Spec.AccesKeyId != "" {
		envVars = append(envVars, v1.EnvVar{Name: "AWS_ACCESS_KEY_ID", Value: s.S3Storage.Spec.AccesKeyId})
		envVars = append(envVars, v1.EnvVar{Name: "AWS_SECRET_ACCESS_KEY", ValueFrom: &v1.EnvVarSource{
			SecretKeyRef: &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: s.S3Storage.Spec.AccessKeyK8sSecret,
				},
				Key: shared.Nvl(s.S3Storage.Spec.AccessKeyK8sSecretKey, "SECRET_ACCESS_KEY"),
			},
		}})
	}

	if s.S3Storage.Spec.RoleArn != "" {
		envVars = append(envVars, v1.EnvVar{Name: "AWS_ROLE_ARN", Value: s.S3Storage.Spec.RoleArn})
	}

	if fixedFileName != nil {
		envVars = append(envVars, v1.EnvVar{Name: "S3_FILE_NAME", Value: *fixedFileName})
	}

	if s.S3Storage.Spec.Endpoint != "" {
		envVars = append(envVars, v1.EnvVar{Name: "S3_ENDPOINT", Value: s.S3Storage.Spec.Endpoint})
	}

	return v1.Container{
		Name:  "s3-upload",
		Image: "amazon/aws-cli",
		Env:   envVars,
		Command: []string{
			path.Join("/", shared.SCRIPTS_VOLUME_NAME, script),
		},
		VolumeMounts: shared.VOLUME_MOUNTS,
	}
}

func (s *S3StorageInfo) BuildUploadContainer(fixedFileName *string) v1.Container {
	return s.BuildContainer(shared.UPLOAD_S3, fixedFileName)
}

func (s *S3StorageInfo) BuildDownloadContainer(fixedFileName *string) v1.Container {
	return s.BuildContainer(shared.DOWNLOAD_S3, fixedFileName)
}
