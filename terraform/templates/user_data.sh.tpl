#!/bin/bash
#
# This script is used to configure and start the gochat service. It sets
# up the environment file with the required command-line arguments and
# starts the gochat Systemd service.
#
######################################################################

set -euo pipefail

cat <<EOF > /etc/default/gochat
GOCHAT_ARGS='-addr=${addr} -dsn="${db_dsn}" -allowed-origins=${allowed_origins} -signing-key=${b64_signing_key}'
EOF

chown gochat:gochat /etc/default/gochat
chmod 640 /etc/default/gochat

systemctl start gochat
