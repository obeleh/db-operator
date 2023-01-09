package shared

type BucketStorageInfo struct {
	StorageTypeName string
	BucketName      string
	Prefix          string
	Region          string
	KeyName         string
	K8sSecret       string
	K8sSecretKey    string
	AssumeRoleName  string
}
