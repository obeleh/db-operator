package controllers

const BACKUP_AZ_BLOBS string = `#!/bin/bash -e
# Only supports up/downloading with service principal
# But should be fairly simple to expand with other methods.
LATEST_DUMP=$(find /pgdump/ -type f | sort | tail -n 1)
LATEST_DUMP_BASE_NAME=$(basename "$LATEST_DUMP")
az login --service-principal -u $AZ_BLOBS_USER -p $AZ_BLOBS_USER_PW --tenant $AZ_BLOBS_TENANT_ID
az storage blob upload \
  --account-name $AZ_BLOBS_STORAGE_ACCOUNT \
  --container-name $AZ_BLOBS_CONTAINER \
  --name $LATEST_DUMP_BASE_NAME \
  --file $LATEST_DUMP
echo "upload done"
`

const BACKUP_POSTGRES string = `#!/bin/bash -e
pg_dump \
	--format=custom \
	--compress=9 \
	--file=/pgdump/${1:-"$DATABASE"_"` + "`" + "date +%Y%m%d%H%M" + "`.dump\"} \\" + `
	--no-owner \
	--no-acl \
	--host=$PGHOST \
	--user=$PGUSER \
	--port=$PGPORT \
	$DATABASE
echo "pg_dump done"
`

const RESTORE_POSTGRES string = `#!/bin/bash -e
LATEST_DUMP=$(find /pgdump/ -type f | sort | tail -n 1)
pg_restore \
	--host=$PGHOST \
	--user=$PGUSER \
	--port=$PGPORT \
	--dbname=$DATABASE \
	--single-transaction \
	--no-owner \
	--no-acl \
	-n public \
	$LATEST_DUMP
echo "pg_restore done"
`

const DOWNLOAD_AZ_BLOBS string = `#!/bin/bash -e
# Only supports up/downloading with service principal
# But should be fairly simple to expand with other methods.
az login --service-principal -u $AZ_BLOBS_USER -p $AZ_BLOBS_USER_PW --tenant $AZ_BLOBS_TENANT_ID
az storage blob download \
  --account-name $AZ_BLOBS_STORAGE_ACCOUNT \
  --container-name $AZ_BLOBS_CONTAINER \
  --name $AZ_BLOBS_FILE_NAME \
  --file /pgdump/$AZ_BLOBS_FILE_NAME
echo "download done"
`

const BACKUP_S3 string = `#!/bin/bash -e
LATEST_DUMP=$(find /pgdump/ -type f | sort | tail -n 1)
LATEST_DUMP_BASE_NAME=$(basename "$LATEST_DUMP")
# aws s3 cp test.txt s3://mybucket/test2.txt
aws s3 cp $LATEST_DUMP s3://$S3_BUCKET/$S3_PREFIX
echo "upload done"
`

const DOWNLOAD_S3 string = `#!/bin/bash -e
aws s3 cp s3://$S3_BUCKET/$S3_PREFIX$S3_FILE_NAME /pgdump/$S3_FILE_NAME
echo "download done"
`

var SCRIPTS_MAP map[string]string = map[string]string{
	"backup_postgres.sh":   BACKUP_POSTGRES,
	"restore_postgres.sh":  RESTORE_POSTGRES,
	"backup_az_blobs.sh":   BACKUP_AZ_BLOBS,
	"download_az_blobs.sh": DOWNLOAD_AZ_BLOBS,
	"backup_s3.sh":         BACKUP_S3,
	"download_s3.sh":       DOWNLOAD_S3,
}
