# db-operator

## Design

### Databases Diagram [ready / beta -ish]

![](./screenshots/databases-diagram.png)

### Backup Restore Diagram

![](./screenshots/backup-restore-diagram.png)


### Dev Requirements

- docker
- kind
- kuttl
- golang

### Running tests

Quick way:

```
docker pull postgres:latest
make kind-cluster
make deploy-test-infra
make deploy
make kuttl-test  # currently only postgres is enabled in these tests
```

Manually:

```
docker pull postgres:latest
make kind-cluster
make deploy-test-infra
make install
# set up port forwards
# see Running the operator on your machine for dns entries in your hosts file
make kuttl-test-cockroachdb-debugmode
make kuttl-test-mysql-debugmode
make kuttl-test-postgres-debugmode
```

### Building / Packaging

```
# up helm chart version in helm/charts/db-operator/Chart.yaml
# git commit
make docker-buildx
make generate-deploys
# git add new tgz file
# git commit and push
```

### Creating new controllers

```
operator-sdk create api --group db-operator --version v1alpha1 --kind <KIND> --resource --controller
```

### Running the operator on your machine with the resources in Kind cluster

add this line to your `/etc/hosts` file:

```
127.0.0.1	localhost postgres.postgres.svc.cluster.local mysql.mysql.svc.cluster.local cockroachdb-public cockroachdb-public.cockroachdb.svc.cluster.local
```

```
make start-test-cluster
kubectl -n postgres port-forward svc/postgres 5432 &
```

If you want to run as binary
```
make run
```

Vscode:
```
{
    "name": "Debug",
    "type": "go",
    "request": "launch",
    "mode": "debug",
    "program": "${workspaceRoot}"
}
```

If you want to run the test scenario's while you're in debug mode:

```
make deploy-test-yamls-postgres
```

cleanup:

```
kind delete cluster
```