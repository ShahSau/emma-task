#!/usr/bin/env bash

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"

# Default to local docker address if not set
APIURL=${APIURL:-http://localhost:8080}
USERNAME=${USERNAME:-u`date +%s`}
EMAIL=${EMAIL:-$USERNAME@mail.com}
PASSWORD=${PASSWORD:-password}

DELAY_REQUEST=${DELAY_REQUEST:-"500"}

echo "------------------------------------------------"
echo "Running Standard API Tests (Conduit)..."
echo "------------------------------------------------"
npx newman run $SCRIPTDIR/Conduit.postman_collection.json \
  --delay-request "$DELAY_REQUEST" \
  --global-var "APIURL=$APIURL" \
  --global-var "USERNAME=$USERNAME" \
  --global-var "EMAIL=$EMAIL" \
  --global-var "PASSWORD=$PASSWORD" \
  "$@" || exit 1

echo ""
echo "------------------------------------------------"
echo "Running Bulk Operations Tests (Import/Export)..."
echo "------------------------------------------------"
# Run the new BulkOps collection
npx newman run $SCRIPTDIR/BulkOps.postman_collection.json \
  --delay-request "$DELAY_REQUEST" \
  --global-var "APIURL=$APIURL" \
  --global-var "USERNAME=$USERNAME" \
  --global-var "EMAIL=$EMAIL" \
  --global-var "PASSWORD=$PASSWORD" \
  "$@"