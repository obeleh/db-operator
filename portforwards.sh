#!/bin/bash

while true; do
    kubectl -n cockroachdb port-forward svc/cockroachdb-public 26257
    sleep 1
done