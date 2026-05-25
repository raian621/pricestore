#!/usr/bin/env bash
if [[ $# -ne 1 ]]; then
  echo "Usage $0 <migration name>"
fi

MIGRATION_NAME=$1
SCRIPT_DIR=$(dirname $0)
CREATE_TIMESTAMP=$(date +%s)
MIGRATION_FILE_PATH="${SCRIPT_DIR}/migrations/${CREATE_TIMESTAMP}-${MIGRATION_NAME}.sql"

mkdir -p $(dirname ${MIGRATION_FILE_PATH})
touch ${MIGRATION_FILE_PATH}
