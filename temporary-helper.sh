#!/usr/bin/env bash
# Temporary Pulse v4 Helper Script for Proxmox VE
# Use this until the official helper script is updated
# TODO: Remove this file once PR to community-scripts/ProxmoxVE is merged

set -euo pipefail

# Colors
BL=$(echo "\033[36m")
RD=$(echo "\033[01;31m")
GN=$(echo "\033[1;92m")
CL=$(echo "\033[m")

# Header
clear
echo -e "${BL}
 ____  _   _ _     ____  _____  __     ___  _   
|  _ \| | | | |   / ___|| ____| \ \   / / || |  
| |_) | | | | |   \___ \|  _|    \ \ / /| || |_ 
|  __/| |_| | |___ ___) | |___    \ V / |__   _|
|_|    \___/|_____|____/|_____|    \_/     |_|  
${CL}"
echo -e "${BL}Temporary Pulse v4 Installation Script${CL}\n"

# Check if running in Proxmox
if [[ ! -f /etc/pve/pve-root-ca.pem ]]; then
    echo -e "${RD}This script must be run on a Proxmox VE host${CL}"
    exit 1
fi

# Default values
CTID=""
CT_NAME="pulse"
MEMORY="1024"
CORES="1"
DISK="8"
BRIDGE="vmbr0"

# Get next available CTID
get_next_ctid() {
    local max_ctid=100
    for ctid in $(pct list | awk 'NR>1 {print $1}' | sort -n); do
        if [ $ctid -ge $max_ctid ]; then
            max_ctid=$((ctid + 1))
        fi
    done
    echo $max_ctid
}

# Prompt for container ID
SUGGESTED_CTID=$(get_next_ctid)
read -p "Enter Container ID [$SUGGESTED_CTID]: " CTID
CTID=${CTID:-$SUGGESTED_CTID}

# Check if CTID already exists
if pct list | grep -q "^$CTID "; then
    echo -e "${RD}Container ID $CTID already exists${CL}"
    exit 1
fi

# Prompt for other settings
read -p "Container Name [$CT_NAME]: " input
CT_NAME=${input:-$CT_NAME}

read -p "Memory (MB) [$MEMORY]: " input
MEMORY=${input:-$MEMORY}

read -p "CPU Cores [$CORES]: " input
CORES=${input:-$CORES}

read -p "Disk Size (GB) [$DISK]: " input
DISK=${input:-$DISK}

# Find Debian 12 template
TEMPLATE=$(ls /var/lib/vz/template/cache/debian-12-standard*.tar.* 2>/dev/null | head -1)
if [ -z "$TEMPLATE" ]; then
    echo -e "${RD}Debian 12 template not found${CL}"
    echo "Download it with: pveam download local debian-12-standard_12.7-1_amd64.tar.zst"
    exit 1
fi

echo -e "\n${BL}Creating LXC Container...${CL}"

# Create container
pct create $CTID "$TEMPLATE" \
    --hostname $CT_NAME \
    --memory $MEMORY \
    --cores $CORES \
    --rootfs local-lvm:$DISK \
    --net0 name=eth0,bridge=$BRIDGE,ip=dhcp \
    --features nesting=1 \
    --unprivileged 1 \
    --onboot 1 \
    --start 1

# Wait for container to start
echo -e "${BL}Waiting for container to start...${CL}"
sleep 5

# Update container and install Pulse
echo -e "\n${BL}Installing Pulse v4...${CL}"
pct exec $CTID -- bash -c "
    apt update && apt upgrade -y
    curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash
"

# Get container IP
CT_IP=$(pct exec $CTID -- ip -4 addr show eth0 | grep inet | awk '{print $2}' | cut -d'/' -f1)

echo -e "\n${GN}✓ Pulse v4 Installation Complete!${CL}"
echo -e "\nContainer ID: ${BL}$CTID${CL}"
echo -e "Container Name: ${BL}$CT_NAME${CL}"
echo -e "IP Address: ${BL}$CT_IP${CL}"
echo -e "\n${BL}Access Pulse at: ${GN}http://$CT_IP:7655${CL}"
echo -e "\n${BL}Note:${CL} Pulse v4 uses port 7655 (not 3000)"
echo -e "No authentication required by default\n"