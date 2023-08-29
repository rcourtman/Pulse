#!/usr/bin/env bash
# A primitive script to install TurnKey LXC templates using basic settings.
# Copyright (c) 2021-2023 tteck
# Author: tteck (tteckster)
# License: MIT
# https://github.com/tteck/Proxmox/raw/main/LICENSE
# bash -c "$(wget -qLO - https://github.com/tteck/Proxmox/raw/main/turnkey/turnkey-core.sh)"

# Setup script environment
PASS="$(openssl rand -base64 8)"
TEMPLATE_SEARCH="debian-11-turnkey-core_17.1-1_amd64.tar.gz"
PCT_OPTIONS="
    -features keyctl=1,nesting=1
    -hostname turnkey-core
    -tags proxmox-helper-scripts
    -onboot 1
    -cores 2
    -memory 2048
    -password $PASS
    -net0 name=eth0,bridge=vmbr0,ip=dhcp
    -unprivileged 1
  "
DEFAULT_PCT_OPTIONS=(
  -arch $(dpkg --print-architecture)
)
function header_info {
clear
cat <<"EOF"
 ______              __ __           _____
/_  __/_ _________  / //_/__ __ __  / ___/__  _______
 / / / // / __/ _ \/ ,< / -_) // / / /__/ _ \/ __/ -_)
/_/  \_,_/_/ /_//_/_/|_|\__/\_, /  \___/\___/_/  \__/
                           /___/
EOF
}
header_info
set -o errexit  #Exit immediately if a pipeline returns a non-zero status
set -o errtrace #Trap ERR from shell functions, command substitutions, and commands from subshell
set -o nounset  #Treat unset variables as an error
set -o pipefail #Pipe will exit with last non-zero status if applicable
shopt -s expand_aliases
alias die='EXIT=$? LINE=$LINENO error_exit'
trap die ERR

function error_exit() {
  trap - ERR
  local DEFAULT='Unknown failure occured.'
  local REASON="\e[97m${1:-$DEFAULT}\e[39m"
  local FLAG="\e[91m[ERROR] \e[93m$EXIT@$LINE"
  msg "$FLAG $REASON" 1>&2
  [ ! -z ${CTID-} ] && cleanup_ctid
  exit $EXIT
}
function warn() {
  local REASON="\e[97m$1\e[39m"
  local FLAG="\e[93m[WARNING]\e[39m"
  msg "$FLAG $REASON"
}
function info() {
  local REASON="$1"
  local FLAG="\e[36m[INFO]\e[39m"
  msg "$FLAG $REASON"
}
function msg() {
  local TEXT="$1"
  echo -e "$TEXT"
}
function cleanup_ctid() {
  if pct status $CTID &>/dev/null; then
    if [ "$(pct status $CTID | awk '{print $2}')" == "running" ]; then
      pct stop $CTID
    fi
    pct destroy $CTID
  fi
}
CTID=$(pvesh get /cluster/nextid)
function select_storage() {
  local CLASS=$1
  local CONTENT
  local CONTENT_LABEL
  case $CLASS in
    container) CONTENT='rootdir'; CONTENT_LABEL='Container';;
    template) CONTENT='vztmpl'; CONTENT_LABEL='Container template';;
    *) false || die "Invalid storage class.";;
  esac

  # Query all storage locations
  local -a MENU
  while read -r line; do
    local TAG=$(echo $line | awk '{print $1}')
    local TYPE=$(echo $line | awk '{printf "%-10s", $2}')
    local FREE=$(echo $line | numfmt --field 4-6 --from-unit=K --to=iec --format %.2f | awk '{printf( "%9sB", $6)}')
    local ITEM="  Type: $TYPE Free: $FREE "
    local OFFSET=2
    if [[ $((${#ITEM} + $OFFSET)) -gt ${MSG_MAX_LENGTH:-} ]]; then
      local MSG_MAX_LENGTH=$((${#ITEM} + $OFFSET))
    fi
    MENU+=( "$TAG" "$ITEM" "OFF" )
  done < <(pvesm status -content $CONTENT | awk 'NR>1')

  # Select storage location
  if [ $((${#MENU[@]}/3)) -eq 0 ]; then            # No storage locations are detected
    warn "'$CONTENT_LABEL' needs to be selected for at least one storage location."
    die "Unable to detect valid storage location."
  elif [ $((${#MENU[@]}/3)) -eq 1 ]; then          # Only one storage location is detected
    printf ${MENU[0]}
  else                                             # More than one storage location is detected
    local STORAGE
    while [ -z "${STORAGE:+x}" ]; do               # Generate graphical menu
      STORAGE=$(whiptail --title "Storage Pools" --radiolist \
      "Which storage pool you would like to use for the ${CONTENT_LABEL,,}?\n\n" \
      16 $(($MSG_MAX_LENGTH + 23)) 6 \
      "${MENU[@]}" 3>&1 1>&2 2>&3) || die "Menu aborted."
    done
    printf $STORAGE
  fi
}

# Test if required variables are set
[[ "${CTID:-}" ]] || die "You need to set 'CTID' variable."

# Test if ID is valid
[ "$CTID" -ge "100" ] || die "ID cannot be less than 100."

# Test if ID is in use
if pct status $CTID &>/dev/null; then
  warn "ID '$CTID' is already in use."
  unset CTID
  die "Cannot use ID that is already in use."
fi

# Get template storage
TEMPLATE_STORAGE=$(select_storage template) || exit
info "Using '$TEMPLATE_STORAGE' for template storage."

# Get container storage
CONTAINER_STORAGE=$(select_storage container) || exit
info "Using '$CONTAINER_STORAGE' for container storage."

# Update LXC template list
msg "Updating LXC template list..."
pveam update >/dev/null

# Get LXC template string
mapfile -t TEMPLATES < <(pveam available -section turnkeylinux | sed -n "s/.*\($TEMPLATE_SEARCH.*\)/\1/p" | sort -t - -k 2 -V)
[ ${#TEMPLATES[@]} -gt 0 ] || die "Unable to find a template when searching for '$TEMPLATE_SEARCH'."
TEMPLATE="${TEMPLATES[-1]}"

# Download LXC template
if ! pveam list $TEMPLATE_STORAGE | grep -q $TEMPLATE; then
  msg "Downloading LXC template (Patience)..."
  pveam download $TEMPLATE_STORAGE $TEMPLATE >/dev/null ||
    die "A problem occured while downloading the LXC template."
fi

PCT_OPTIONS=( ${PCT_OPTIONS[@]:-${DEFAULT_PCT_OPTIONS[@]}} )
[[ " ${PCT_OPTIONS[@]} " =~ " -rootfs " ]] || PCT_OPTIONS+=( -rootfs $CONTAINER_STORAGE:${PCT_DISK_SIZE:-8} )

# Create LXC
msg "Creating LXC container..."
pct create $CTID ${TEMPLATE_STORAGE}:vztmpl/${TEMPLATE} ${PCT_OPTIONS[@]} >/dev/null ||
  die "A problem occured while trying to create container."

# Success message
msg "Starting LXC Container..."
pct start "$CTID"
info "LXC container '$CTID' was successfully created."
echo "TurnKey Core Password" >>~/turnkey-core.creds # file is located in the Proxmox root directory
echo $PASS >>~/turnkey-core.creds #run `cat turnkey-core.creds` in the Proxmox shell
info "Proceed to the LXC console to complete the setup."
info "login: root"
info "password: $PASS"
