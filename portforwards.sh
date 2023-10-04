#!/bin/bash

usage() {
    echo "Usage: $0 <command>"
    echo "Available commands:"
    echo "  crdb     Start CockroachDB port forwarding"
    echo "  pg       Start PostgreSQL port forwarding"
    echo "  mysql    Start MySQL port forwarding"
}

port_forward_crdb() {
    while true; do
        kubectl -n cockroachdb port-forward svc/cockroachdb-public 26257 || echo "Error: Failed to port forward CockroachDB"
        sleep 1
    done
}

port_forward_pg() {
    while true; do
        kubectl -n postgres port-forward svc/postgres 5432 || echo "Error: Failed to port forward PostgreSQL"
        sleep 1
    done
}

port_forward_mysql() {
    while true; do
        kubectl -n mysql port-forward svc/mysql 3306 || echo "Error: Failed to port forward MySQL"
        sleep 1
    done
}

if [ "$#" -eq 0 ]; then
    usage
    exit 1
fi

command="$1"
shift

case $command in
    crdb)
        port_forward_crdb "$@"
        ;;
    pg)
        port_forward_pg "$@"
        ;;
    mysql)
        port_forward_mysql "$@"
        ;;
    help|--help|-h)
        usage
        ;;
    *)
        echo "Invalid command. Use 'help' for more information."
        exit 1
        ;;
esac
