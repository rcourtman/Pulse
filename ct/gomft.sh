#!/usr/bin/env bash
source <(curl -fsSL https://raw.githubusercontent.com/community-scripts/ProxmoxVE/main/misc/build.func)
# Copyright (c) 2021-2025 community-scripts ORG
# Author: Slaviša Arežina (tremor021)
# License: MIT | https://github.com/community-scripts/ProxmoxVE/raw/main/LICENSE
# Source: https://github.com/StarFleetCPTN/GoMFT

APP="GoMFT"
var_tags="${var_tags:-backup}"
var_cpu="${var_cpu:-1}"
var_ram="${var_ram:-512}"
var_disk="${var_disk:-4}"
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

  if [[ ! -d "/opt/gomft" ]]; then
    msg_error "No ${APP} Installation Found!"
    exit
  fi
  if ! dpkg -l | grep -q "^ii.*build-essential"; then
    $STD apt-get install -y build-essential
  fi
  if [[ ! -f "/usr/bin/node" ]]; then
    mkdir -p /etc/apt/keyrings
    curl -fsSL "https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key" | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg
    echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_22.x nodistro main" >/etc/apt/sources.list.d/nodesource.list
    $STD apt-get update
    $STD apt-get install -y nodejs
  fi
  RELEASE=$(curl -fsSL "https://api.github.com/repos/StarFleetCPTN/GoMFT/releases/latest" | grep "tag_name" | awk '{print substr($2, 3, length($2)-4) }')
  if [[ "${RELEASE}" != "$(cat /opt/${APP}_version.txt)" ]] || [[ ! -f /opt/${APP}_version.txt ]]; then
    msg_info "Stopping $APP"
    systemctl stop gomft
    msg_ok "Stopped $APP"

    msg_info "Updating $APP to ${RELEASE}"
    if ! command -v git >/dev/null 2>&1; then
      $STD apt-get install -y git
    fi
    rm -f /opt/gomft/gomft
    temp_file=$(mktemp)
    curl -fsSL "https://github.com/StarFleetCPTN/GoMFT/archive/refs/tags/v${RELEASE}.tar.gz" -o "$temp_file"
    tar -xzf "$temp_file"
    cp -rf "GoMFT-${RELEASE}"/* /opt/gomft/
    cd /opt/gomft
    $STD npm install
    $STD npm run build
    TEMPL_VERSION="$(awk '/github.com\/a-h\/templ/{print $2}' go.mod)"
    $STD go install github.com/a-h/templ/cmd/templ@${TEMPL_VERSION}
    # dirty hack to fix templ
    cat <<'EOF' >/opt/gomft/components/file_metadata/search/file_metadata_search_content.templ
package search

import (
    "context"
    "github.com/starfleetcptn/gomft/components/file_metadata"
    "github.com/starfleetcptn/gomft/components/file_metadata/list"
)

templ FileMetadataSearchContent(ctx context.Context, data file_metadata.FileMetadataSearchData) {
    <!-- Search Results -->
    <div id="search-results">
        if len(data.Files) > 0 {
            @list.FileMetadataListPartial(ctx, file_metadata.FileMetadataListData{
                Files:      data.Files,
                Page:       data.Page,
                Limit:      data.Limit,
                TotalCount: data.TotalCount,
                TotalPages: data.TotalPages,
                Filter:     data.Filter,
                SortBy:     data.SortBy,
                SortDir:    data.SortDir,
            }, "/files/search/partial", "#search-results-container")
        } else {
            <div class="p-6 text-center text-gray-500 dark:text-gray-400">
                <svg class="mx-auto mb-4 w-12 h-12 text-gray-400" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
                </svg>
                <p>No files found matching your search criteria.</p>
            </div>
        }
    </div>
}
EOF
    $STD "$HOME"/go/bin/templ generate
    export CGO_ENABLED=1
    export GOOS=linux
    $STD go build -o gomft
    chmod +x /opt/gomft/gomft
    echo "${RELEASE}" >/opt/${APP}_version.txt
    msg_ok "Updated $APP to ${RELEASE}"

    msg_info "Cleaning Up"
    rm -f "$temp_file"
    rm -rf "$HOME/GoMFT-v.${RELEASE}/"
    msg_ok "Cleanup Complete"

    msg_info "Starting $APP"
    systemctl start gomft
    msg_ok "Started $APP"

    msg_ok "Update Successful"
  else
    msg_ok "No update required. ${APP} is already at ${RELEASE}"
  fi
  exit
}

start
build_container
description

msg_ok "Completed Successfully!\n"
echo -e "${CREATING}${GN}${APP} setup has been successfully initialized!${CL}"
echo -e "${INFO}${YW} Access it using the following URL:${CL}"
echo -e "${TAB}${GATEWAY}${BGN}http://${IP}:8080${CL}"
