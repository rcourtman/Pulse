#!/usr/bin/env bash
source <(curl -fsSL https://raw.githubusercontent.com/community-scripts/ProxmoxVE/main/misc/build.func)
# Copyright (c) 2021-2025 community-scripts ORG
# Author: Omar Minaya | MickLesk (CanbiZ)
# License: MIT | https://github.com/community-scripts/ProxmoxVE/raw/main/LICENSE
# Source: https://github.com/C4illin/ConvertX

APP="ConvertX"
var_tags="${var_tags:-converter}"
var_cpu="${var_cpu:-2}"
var_ram="${var_ram:-4096}"
var_disk="${var_disk:-20}"
var_os="${var_os:-debian}"
var_version="${var_version:-12}"
var_unprivileged="${var_unprivileged:-1}"

header_info "$APP"
variables
color
catch_errors

function update_script() {
  header_info
  check_container_storage
  check_container_resources
  if [[ ! -d /var ]]; then
    msg_error "No ${APP} Installation Found!"
    exit
  fi
  RELEASE=$(curl -fsSL https://api.github.com/repos/C4illin/ConvertX/releases/latest | grep "tag_name" | awk '{print substr($2, 2, length($2)-3) }')
  if [[ "${RELEASE}" != "$(cat ~/.convertx 2>/dev/null)" ]] || [[ ! -f ~/.convertx ]]; then
    msg_info "Stopping $APP"
    systemctl stop convertx
    msg_ok "Stopped $APP"

    msg_info "Move data-Folder"
    if [[ -d /opt/convertx/data ]]; then
      mv /opt/convertx/data /opt/data
    fi
    msg_ok "Moved data-Folder"

    fetch_and_deploy_gh_release "ConvertX" "C4illin/ConvertX" "tarball" "latest" "/opt/convertx"

    msg_info "Updating $APP to v${RELEASE}"
    if [[ -d /opt/data ]]; then
      mv /opt/data /opt/convertx/data
    fi
    cd /opt/convertx
    $STD bun install
    msg_ok "Updated $APP to v${RELEASE}"

    msg_info "Starting $APP"
    systemctl start convertx
    msg_ok "Started $APP"

    msg_ok "Update Successful"
  else
    msg_ok "No update required. ${APP} is already at v${RELEASE}"
  fi
  exit
}

start
build_container
description

msg_ok "Completed Successfully!\n"
echo -e "${CREATING}${GN}${APP} setup has been successfully initialized!${CL}"
echo -e "${INFO}${YW} Access it using the following URL:${CL}"
echo -e "${TAB}${GATEWAY}${BGN}http://${IP}:3000${CL}"
