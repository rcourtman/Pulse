#!/usr/bin/env bash

# Copyright (c) 2021-2025 community-scripts ORG
# Author: MickLesk (CanbiZ)
# License: MIT | https://github.com/community-scripts/ProxmoxVE/raw/main/LICENSE
# Source: https://nodered.org/

source /dev/stdin <<<"$FUNCTIONS_FILE_PATH"
color
verb_ip6
catch_errors
setting_up_container
network_check
update_os

msg_info "Installing Dependencies"
$STD apk add --no-cache \
  git \
  nodejs \
  npm
msg_ok "Installed Dependencies"

msg_info "Creating Node-RED User"
adduser -D -H -s /sbin/nologin -G users nodered
msg_ok "Created Node-RED User"

msg_info "Installing Node-RED"
$STD npm install -g --unsafe-perm node-red
msg_ok "Installed Node-RED"

msg_info "Creating /home/nodered"
mkdir -p /home/nodered
chown -R nodered:users /home/nodered
chmod 750 /home/nodered
msg_ok "Created /home/nodered"

motd_ssh
customize
