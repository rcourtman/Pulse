#!/usr/bin/env bash

# Copyright (c) 2021-2023 tteck
# Author: tteck (tteckster)
# License: MIT
# https://github.com/tteck/Proxmox/raw/main/LICENSE

source /dev/stdin <<< "$FUNCTIONS_FILE_PATH"
color
verb_ip6
catch_errors
setting_up_container
network_check
update_os

msg_info "Installing Dependencies"
$STD apt-get install -y curl
$STD apt-get install -y sudo
$STD apt-get install -y mc
$STD apt-get install -y git
$STD apt-get install -y pip
msg_ok "Installed Dependencies"

msg_info "Installing Tautulli"
cd /opt
git clone https://github.com/Tautulli/Tautulli.git
python3 -m pip install -q -r /opt/Tautulli/requirements.txt
msg_ok "Installed Tautulli"

msg_info "Creating Service"
cat <<EOF >/etc/systemd/system/tautulli.service
[Unit]
Description=Tautulli
After=syslog.target network.target

[Service]
WorkingDirectory=/opt/Tautulli/
Restart=on-failure
RestartSec=5
Type=simple
ExecStart=/usr/bin/python3 /opt/Tautulli/Tautulli.py
KillSignal=SIGINT
TimeoutStopSec=20
SyslogIdentifier=tautulli

[Install]
WantedBy=multi-user.target
EOF
systemctl enable -q --now tautulli
msg_ok "Created Service"

motd_ssh
root

msg_info "Cleaning up"
$STD apt-get autoremove
$STD apt-get autoclean
msg_ok "Cleaned"
