#!/bin/bash

crdb() {
    while true; do
        kubectl -n cockroachdb port-forward svc/cockroachdb-public 26257
        sleep 1
    done
}
pg() {
    while true; do
        kubectl -n postgres port-forward svc/postgres 5432
        sleep 1
    done
}
mysql() {
    while true; do
        kubectl -n mysql port-forward svc/mysql 3306
        sleep 1
    done
}

crdb &
pg &
mysql &