#!/usr/bin/env bash

# Copyright (c) 2021-2024 tteck
# Author: tteck (tteckster)
# License: MIT
# https://github.com/community-scripts/ProxmoxVE/raw/main/LICENSE

function header_info {
  clear
  cat <<"EOF"
    __ __                     __   ________
   / //_/__  _________  ___  / /  / ____/ /__  ____ _____
  / ,< / _ \/ ___/ __ \/ _ \/ /  / /   / / _ \/ __ `/ __ \
 / /| /  __/ /  / / / /  __/ /  / /___/ /  __/ /_/ / / / /
/_/ |_\___/_/  /_/ /_/\___/_/   \____/_/\___/\__,_/_/ /_/

EOF
}

# Color variables for output
YW=$(echo "\033[33m")
RD=$(echo "\033[01;31m")
GN=$(echo "\033[1;92m")
CL=$(echo "\033[m")
BFR="\\r\\033[K"
HOLD="-"
CM="${GN}✓${CL}"

# Functions for logging messages
function msg_info() {
  local msg="$1"
  echo -ne " ${HOLD} ${YW}${msg}..."
}

function msg_ok() {
  local msg="$1"
  echo -e "${BFR} ${CM} ${GN}${msg}${CL}"
}

# Detect current kernel
current_kernel=$(uname -r)

# Detect all installed kernels except the current one
available_kernels=$(dpkg --list | grep 'kernel-.*-pve' | awk '{print $2}' | grep -v "$current_kernel" | sort -V)

header_info

# If no old kernels are available, exit with a message
if [ -z "$available_kernels" ]; then
  whiptail --backtitle "Proxmox VE Helper Scripts" --title "No Old Kernels" \
    --msgbox "It appears there are no old kernels on your system.\nCurrent kernel: $current_kernel" 10 68
  echo "Exiting..."
  sleep 2
  clear
  exit
fi

# Prepare kernel options for selection
KERNEL_MENU=()
while read -r TAG ITEM; do
  OFFSET=2
  MSG_MAX_LENGTH=$((MSG_MAX_LENGTH < ${#ITEM} + OFFSET ? ${#ITEM} + OFFSET : MSG_MAX_LENGTH))
  KERNEL_MENU+=("$TAG" "$ITEM " "OFF")
done < <(echo "$available_kernels")

# Display checklist to select kernels for removal
remove_kernels=$(whiptail --backtitle "Proxmox VE Helper Scripts" \
  --title "Current Kernel: $current_kernel" \
  --checklist "\nSelect kernels to remove:\n" \
  16 $((MSG_MAX_LENGTH + 58)) 6 "${KERNEL_MENU[@]}" 3>&1 1>&2 2>&3 | tr -d '"') || exit

# Exit if no kernel was selected
[ -z "$remove_kernels" ] && {
  whiptail --backtitle "Proxmox VE Helper Scripts" --title "No Kernel Selected" \
    --msgbox "It appears no kernel was selected." 10 68
  echo "Exiting..."
  sleep 2
  clear
  exit
}

# Confirm removal
whiptail --backtitle "Proxmox VE Helper Scripts" --title "Remove Kernels" \
  --yesno "Would you like to remove the $(echo $remove_kernels | awk '{print NF}') selected kernels?" 10 68 || exit

# Process kernel removal
msg_info "Removing ${RD}$(echo $remove_kernels | awk '{print NF}') ${YW}old kernels${CL}"
for kernel in $remove_kernels; do
  if [[ $kernel == *"-signed" ]]; then
    # Handle signed kernels with dependencies
    touch /please-remove-proxmox-ve  # Temporarily bypass Proxmox warnings
    if sudo apt-get purge -y "$kernel" >/dev/null 2>&1; then
      msg_ok "Removed kernel: $kernel"
    else
      msg_info "Failed to remove kernel: $kernel. Check dependencies or manual removal."
    fi
    rm -f /please-remove-proxmox-ve  # Clean up bypass file
  else
    # Standard kernel removal
    if sudo apt-get purge -y "$kernel" >/dev/null 2>&1; then
      msg_ok "Removed kernel: $kernel"
    else
      msg_info "Failed to remove kernel: $kernel. Check dependencies or manual removal."
    fi
  fi
  sleep 1
done

# Update GRUB configuration
msg_info "Updating GRUB"
if /usr/sbin/update-grub >/dev/null 2>&1; then
  msg_ok "GRUB updated successfully"
else
  msg_info "Failed to update GRUB"
fi

# Completion message
msg_info "Exiting"
sleep 2
msg_ok "Finished"
