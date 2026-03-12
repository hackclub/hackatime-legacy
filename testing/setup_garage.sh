#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/compose.yml"
BUCKET_NAME="hackatime-test"
KEY_NAME="test-key"

garage_cli() {
    docker compose -f "$COMPOSE_FILE" exec -T garage /garage "$@" 2>&1
}

echo "Waiting for Garage to be ready..." >&2
for i in $(seq 1 30); do
    if garage_cli status | grep -q "HEALTHY NODES"; then
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "Garage did not become ready in time" >&2
        exit 1
    fi
    sleep 1
done

NODE_ID=$(garage_cli status | awk '/HEALTHY NODES/{found=1; next} found && /^[0-9a-f]{16}/{print $1; exit}')
if [ -z "$NODE_ID" ]; then
    echo "Could not extract node ID from garage status" >&2
    exit 1
fi
echo "Node ID: $NODE_ID" >&2

garage_cli layout assign -z dc1 -c 1G "$NODE_ID" >&2 || true
garage_cli layout apply --version 1 >&2 || true

garage_cli bucket create "$BUCKET_NAME" >&2 || true

KEY_OUTPUT=$(garage_cli key create "$KEY_NAME" 2>&1 || garage_cli key info "$KEY_NAME" 2>&1)

KEY_ID=$(echo "$KEY_OUTPUT" | grep "Key ID:" | head -1 | awk '{print $NF}')
SECRET=$(echo "$KEY_OUTPUT" | grep "Secret key:" | head -1 | awk '{print $NF}')

if [ -z "$KEY_ID" ] || [ -z "$SECRET" ]; then
    echo "Failed to extract key info from:" >&2
    echo "$KEY_OUTPUT" >&2
    exit 1
fi

garage_cli bucket allow --read --write --owner "$BUCKET_NAME" --key "$KEY_NAME" >&2 || true

echo "KEY_ID=$KEY_ID"
echo "SECRET_KEY=$SECRET"
echo "Garage setup complete." >&2
