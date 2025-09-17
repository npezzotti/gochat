[Unit]
Description=Go Chat
After=network.target

[Service]
Type=simple
User=gochat
WorkingDirectory=/opt/gochat
ExecStart=/opt/gochat/gochat $GOCHAT_ARGS
Restart=always
EnvironmentFile=/etc/default/gochat

[Install]
WantedBy=multi-user.target
