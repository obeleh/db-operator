package shared

const BACKUP_AZ_BLOBS_SCRIPT string = `#!/bin/bash -e
# Only supports up/downloading with service principal
# But should be fairly simple to expand with other methods.
LATEST_BACKUP=$(find /backups/ -type f | sort | tail -n 1)
LATEST_BACKUP_BASE_NAME=$(basename "$LATEST_BACKUP")
az login --service-principal -u $AZ_BLOBS_USER -p $AZ_BLOBS_USER_PW --tenant $AZ_BLOBS_TENANT_ID
az storage blob upload \
  --account-name $AZ_BLOBS_STORAGE_ACCOUNT \
  --container-name $AZ_BLOBS_CONTAINER \
  --name $LATEST_BACKUP_BASE_NAME \
  --file $LATEST_BACKUP
echo "upload done"
`

const BACKUP_POSTGRES_SCRIPT string = `#!/bin/bash -e
pg_dump \
	--format=custom \
	--compress=9 \
	--file=/backups/${1:-"$DATABASE"_"` + "`" + "date +%Y%m%d%H%M" + "`.dump\"} \\" + `
	--no-owner \
	--no-acl \
	--host=$PGHOST \
	--user=$PGUSER \
	--port=$PGPORT \
	$DATABASE
echo "pg_dump done"
`

const RESTORE_POSTGRES_SCRIPT string = `#!/bin/bash -e
LATEST_BACKUP=$(find /backups/ -type f | sort | tail -n 1)
pg_restore \
	--host=$PGHOST \
	--user=$PGUSER \
	--port=$PGPORT \
	--dbname=$DATABASE \
	--single-transaction \
	--no-owner \
	--no-acl \
	-n public \
	$LATEST_BACKUP
echo "pg_restore done"
`

const BACKUP_MYSQL_SCRIPT string = `#!/bin/bash -e
mysqldump \
	-u $MYSQL_USER \
	-h $MYSQL_HOST \
	-d $MYSQL_DATABASE \
	> /backups/${1:-"$MYSQL_DATABASE"_"` + "`" + "date +%Y%m%d%H%M" + "`.sql\"}" + `
echo "mysqldump done"
`

const RESTORE_MYSQL_SCRIPT string = `#!/bin/bash -e
LATEST_BACKUP=$(find /backups/ -type f | sort | tail -n 1)
mysql -u $MYSQL_USER -h $MYSQL_HOST $MYSQL_DATABASE < $LATEST_BACKUP
echo "mysql restore done"
`

const DOWNLOAD_AZ_BLOBS_SCRIPT string = `#!/bin/bash -e
# Only supports up/downloading with service principal
# But should be fairly simple to expand with other methods.
az login --service-principal -u $AZ_BLOBS_USER -p $AZ_BLOBS_USER_PW --tenant $AZ_BLOBS_TENANT_ID
az storage blob download \
  --account-name $AZ_BLOBS_STORAGE_ACCOUNT \
  --container-name $AZ_BLOBS_CONTAINER \
  --name $AZ_BLOBS_FILE_NAME \
  --file /backups/$AZ_BLOBS_FILE_NAME
echo "download done"
`

const UPLOAD_S3_SCRIPT string = `#!/bin/bash -e
LATEST_BACKUP=$(find /backups/ -type f | sort | tail -n 1)
LATEST_BACKUP_BASE_NAME=$(basename "$LATEST_BACKUP")
# aws s3 cp test.txt s3://mybucket/test2.txt
aws s3 cp $LATEST_BACKUP s3://$S3_BUCKET_NAME/$S3_PREFIX
echo "upload done"
`

const DOWNLOAD_S3_SCRIPT string = `#!/bin/bash -e
if [[ -z "$S3_FILE_NAME" ]]; then
	# if not set find the latest file
	S3_FILE_NAME=$(aws s3 ls $S3_BUCKET_NAME/$S3_PREFIX | sort | tail -n 1 | awk '{print $4}')
fi

aws s3 cp s3://$S3_BUCKET_NAME/$S3_PREFIX$S3_FILE_NAME /backups/$S3_FILE_NAME
echo "download done"
`
const SCRIPTS_VOLUME_NAME = "scripts"
const BACKUP_VOLUME_NAME = "backups"

const BACKUP_POSTGRES string = "backup_postgres.sh"
const RESTORE_POSTGRES string = "restore_postgres.sh"
const BACKUP_MYSQL string = "backup_mysql.sh"
const RESTORE_MYSQL string = "restore_mysql.sh"
const BACKUP_AZ_BLOBS string = "backup_az_blobs.sh"
const DOWNLOAD_AZ_BLOBS string = "download_az_blobs.sh"
const UPLOAD_S3 string = "upload_s3.sh"
const DOWNLOAD_S3 string = "download_s3.sh"

var SCRIPTS_MAP map[string]string = map[string]string{
	BACKUP_POSTGRES:   BACKUP_POSTGRES_SCRIPT,
	RESTORE_POSTGRES:  RESTORE_POSTGRES_SCRIPT,
	BACKUP_MYSQL:      BACKUP_MYSQL_SCRIPT,
	RESTORE_MYSQL:     RESTORE_MYSQL_SCRIPT,
	BACKUP_AZ_BLOBS:   BACKUP_AZ_BLOBS_SCRIPT,
	DOWNLOAD_AZ_BLOBS: DOWNLOAD_AZ_BLOBS_SCRIPT,
	UPLOAD_S3:         UPLOAD_S3_SCRIPT,
	DOWNLOAD_S3:       DOWNLOAD_S3_SCRIPT,
}
