#!/usr/bin/env bash

# Copyright (c) 2021-2025 community-scripts ORG
# Author: Slaviša Arežina (tremor021)
# License: MIT | https://github.com/community-scripts/ProxmoxVE/raw/main/LICENSE
# Source: https://github.com/StarFleetCPTN/GoMFT

source /dev/stdin <<<"$FUNCTIONS_FILE_PATH"
color
verb_ip6
catch_errors
setting_up_container
network_check
update_os

msg_info "Installing Dependencies"
$STD apt-get install -y \
  sqlite3 \
  rclone \
  tzdata \
  ca-certificates \
  build-essential \
  git
msg_ok "Installed Dependencies"

setup_go
NODE_VERSION="22" setup_nodejs

msg_info "Setup ${APPLICATION} (Patience)"
temp_file=$(mktemp)
RELEASE=$(curl -fsSL https://api.github.com/repos/StarFleetCPTN/GoMFT/releases/latest | grep "tag_name" | awk '{print substr($2, 3, length($2)-4) }')
curl -fsSL "https://github.com/StarFleetCPTN/GoMFT/archive/refs/tags/v${RELEASE}.tar.gz" -o "$temp_file"
tar -xzf "$temp_file"
mv GoMFT-"${RELEASE}"/ /opt/gomft
cd /opt/gomft
TEMPL_VERSION="$(awk '/github.com\/a-h\/templ/{print $2}' go.mod)"
$STD go install github.com/a-h/templ/cmd/templ@${TEMPL_VERSION}
cp /opt/gomft/components/file_metadata/search/file_metadata_search_content.templ{,.bak}
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
$STD go build -o gomft main.go
chmod +x /opt/gomft/gomft
JWT_SECRET_KEY=$(openssl rand -base64 24 | tr -d '/+=')

cat <<EOF >/opt/gomft/.env
SERVER_ADDRESS=:8080
DATA_DIR=/opt/gomft/data/gomft
BACKUP_DIR=/opt/gomft/data/gomft/backups
JWT_SECRET=$JWT_SECRET_KEY
BASE_URL=http://localhost:8080

# Email configuration
EMAIL_ENABLED=false
EMAIL_HOST=smtp.example.com
EMAIL_PORT=587
EMAIL_FROM_EMAIL=gomft@example.com
EMAIL_FROM_NAME=GoMFT
EMAIL_REPLY_TO=
EMAIL_ENABLE_TLS=true
EMAIL_REQUIRE_AUTH=true
EMAIL_USERNAME=smtp_username
EMAIL_PASSWORD=smtp_password
EOF

echo "${RELEASE}" >/opt/"${APPLICATION}"_version.txt
msg_ok "Setup ${APPLICATION}"

msg_info "Creating Service"
cat <<EOF >/etc/systemd/system/gomft.service
[Unit]
Description=GoMFT Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/gomft
ExecStart=/opt/gomft/./gomft
Restart=always

[Install]
WantedBy=multi-user.target
EOF
systemctl enable -q --now gomft
msg_ok "Created Service"

motd_ssh
customize

msg_info "Cleaning up"
rm -f "$temp_file"
$STD apt-get -y autoremove
$STD apt-get -y autoclean
msg_ok "Cleaned"

motd_ssh
customize
