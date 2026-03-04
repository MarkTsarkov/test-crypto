#!/bin/bash
source .env

export MIGRATION_DSN="host=postgres port=5432 dbname=$POSTGRES_DB user=$POSTGRES_USER password=$POSTGRES_PASSWORD sslmode=disable"

sleep 2
goose -dir . postgres "${MIGRATION_DSN}" up -v