#!/usr/bin/env bash
set -euo pipefail

KEY_DIR="$(dirname "$0")/.ssh"
KEY_FILE="$KEY_DIR/id_ed25519"

# Generate keypair if not present
if [ ! -f "$KEY_FILE" ]; then
    mkdir -p "$KEY_DIR"
    ssh-keygen -t ed25519 -f "$KEY_FILE" -N "" -q
    echo "Generated SSH keypair at $KEY_FILE"
fi

PUB_KEY=$(cat "$KEY_FILE.pub")

# Inject public key into each container and verify connectivity
for port in 2201 2202 2203; do
    container="bolt-test-node$((port - 2200))"
    echo "Setting up SSH key on $container (port $port)..."
    docker exec "$container" bash -c "echo '$PUB_KEY' > /home/testuser/.ssh/authorized_keys && chown testuser:testuser /home/testuser/.ssh/authorized_keys && chmod 600 /home/testuser/.ssh/authorized_keys"

    # Wait for sshd to be ready and verify connectivity
    for i in $(seq 1 10); do
        if ssh -i "$KEY_FILE" -o StrictHostKeyChecking=no -o ConnectTimeout=2 -p "$port" testuser@127.0.0.1 echo "OK" 2>/dev/null; then
            echo "  SSH connectivity verified on port $port"
            break
        fi
        if [ "$i" -eq 10 ]; then
            echo "  ERROR: SSH connectivity failed on port $port" >&2
            exit 1
        fi
        sleep 1
    done
done

echo "All nodes ready."
