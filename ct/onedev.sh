#!/usr/bin/env bash
source <(curl -s https://raw.githubusercontent.com/community-scripts/ProxmoxVE/main/misc/build.func)
# Copyright (c) 2021-2024 community-scripts ORG
# Author: kristocopani
# License: MIT
# https://github.com/community-scripts/ProxmoxVE/raw/main/LICENSE

function header_info {
clear
cat <<"EOF"
   ____             ____           
  / __ \____  ___  / __ \___ _   __
 / / / / __ \/ _ \/ / / / _ \ | / /
/ /_/ / / / /  __/ /_/ /  __/ |/ / 
\____/_/ /_/\___/_____/\___/|___/  
                                    
EOF
}
header_info
echo -e "Loading..."
APP="OneDev"
var_disk="4"
var_cpu="2"
var_ram="2048"
var_os="debian"
var_version="12"
variables
color
catch_errors

function default_settings() {
  CT_TYPE="1"
  PW=""
  CT_ID=$NEXTID
  HN=$NSAPP
  DISK_SIZE="$var_disk"
  CORE_COUNT="$var_cpu"
  RAM_SIZE="$var_ram"
  BRG="vmbr0"
  NET="dhcp"
  GATE=""
  APT_CACHER=""
  APT_CACHER_IP=""
  DISABLEIP6="no"
  MTU=""
  SD=""
  NS=""
  MAC=""
  VLAN=""
  SSH="no"
  VERB="no"
  echo_default
}
function update_script() {
header_info
check_container_storage
check_container_resources

  if [[ ! -f /etc/systemd/system/onedev.service ]]; then
    msg_error "No ${APP} Installation Found!"
    exit
  fi
  GITHUB_RELEASE=$(curl -s https://api.github.com/repos/theonedev/onedev/releases/latest | grep "tag_name" | awk '{print substr($2, 3, length($2)-4) }')
  if [[ ! -f /opt/${APP}_version.txt ]] || [[ "${GITHUB_RELEASE}" != "$(cat /opt/${APP}_version.txt)" ]]; then
    msg_info "Stopping Service"
    systemctl stop onedev
    msg_ok "Stopped Service"

    msg_info "Updating ${APP} to v${GITHUB_RELEASE}"
    cd /opt
    wget -q https://code.onedev.io/onedev/server/~site/onedev-latest.tar.gz
    tar -xzf onedev-latest.tar.gz
    /opt/onedev-latest/bin/upgrade.sh /opt/onedev >/dev/null
    RELEASE=$(cat /opt/onedev/release.properties | grep "version" | cut -d'=' -f2)
    echo "${RELEASE}" >"/opt/${APP}_version.txt"
    msg_ok "Updated ${APP} to v${RELEASE}"

    msg_info "Starting Service"
    systemctl start onedev
    msg_ok "Started Service"

    msg_info "Cleaning up"
    rm -rf /opt/onedev-latest
    rm -rf /opt/onedev-latest.tar.gz
    msg_ok "Cleaned"
    msg_ok "Updated Successfully"
  else
    msg_ok "No update required. ${APP} is already at v${RELEASE}."
  fi
  exit
}

start
build_container
description

msg_ok "Completed Successfully!\n"
echo -e "${APP} should be reachable by going to the following URL.
         ${BL}http://${IP}:6610${CL} \n"