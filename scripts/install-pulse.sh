#!/bin/bash

set -euo pipefail

# Pulse for Proxmox VE LXC Installation Script
# This script automates the installation and setup of Pulse within a Proxmox LXC container.

# --- Configuration ---
NODE_MAJOR_VERSION=${NODE_MAJOR_VERSION:-20}
PULSE_DIR="${PULSE_DIR:-/opt/pulse-monitor}"
PULSE_USER="${PULSE_USER:-pulse}"
SERVICE_NAME="${SERVICE_NAME:-pulse-monitor.service}"
SCRIPT_NAME="install-pulse.sh"
LOG_FILE="/var/log/pulse_update.log"
CRON_IDENTIFIER="# Pulse-Auto-Update ($SCRIPT_NAME)"
SCRIPT_ABS_PATH=""

# --- Flags ---
MODE_UPDATE=false
MODE_YES=false
MODE_LOG=false
LOG_TARGET=""
MODE_HELP=false

# --- Centralized Prompt Functions ---
prompt_yes_no() {
  local question=$1
  local default=${2:-Y} # Default to Y if not provided
  local answer

  if [ "$default" = "Y" ] || [ "$default" = "y" ]; then
    prompt_suffix="[Y/n]"
  else
    prompt_suffix="[y/N]"
  fi

  while true; do
    echo "DEBUG: Inside prompt_yes_no, before read"
    read -p "$question $prompt_suffix: " answer
    answer=${answer:-$default} # Use default if user just presses Enter
    case "$answer" in
      [Yy]|[Yy][Ee][Ss] ) return 0;; # Return success (true in shell)
      [Nn]|[Nn][Oo]     ) return 1;; # Return failure (false in shell)
      * ) echo "Please answer yes or no.";;
    esac
  done
}

prompt_value() {
  local prompt="$1"
  local var_name="$2"
  local default="$3"
  local silent="${4:-false}"
  if $MODE_YES && [ -n "${!var_name:-}" ]; then
    return 0
  fi
  while true; do
    if $silent; then
      read -rsp "$prompt" value; echo
    else
      read -rp "$prompt" value
    fi
    value="${value:-$default}"
    if [ -n "$value" ]; then
      eval "$var_name=\"\$value\""
      break
    fi
  done
}

# --- Logging ---
log() {
  if $MODE_LOG; then
    echo -e "$1" | tee -a "$LOG_TARGET"
  else
    echo -e "$1"
  fi
}

print_info()    { log "\033[1;34m[INFO]\033[0m $1"; }
print_success(){ log "\033[1;32m[SUCCESS]\033[0m $1"; }
print_warning(){ log "\033[1;33m[WARNING]\033[0m $1"; }
print_error()  { log "\033[1;31m[ERROR]\033[0m $1" >&2; }

# --- Help/Usage ---
print_help() {
  cat <<EOF
Usage: $0 [options]
Options:
  --update         Run in non-interactive update mode
  -y, --yes        Assume yes to all prompts (non-interactive)
  --log <file>     Log output to <file>
  -h, --help       Show this help message
Environment variables can be used to pre-fill config (e.g. PROXMOX_HOST, PROXMOX_TOKEN_ID, ...)
EOF
}

# --- Argument Parsing ---
while [[ "$#" -gt 0 ]]; do
  case $1 in
    --update) MODE_UPDATE=true ;;
    -y|--yes) MODE_YES=true ;;
    --log) MODE_LOG=true; shift; LOG_TARGET="$1" ;;
    -h|--help) MODE_HELP=true ;;
    *) print_error "Unknown parameter passed: $1"; print_help; exit 1 ;;
  esac
  shift
done
if $MODE_HELP; then print_help; exit 0; fi
if $MODE_LOG && [ -n "$LOG_TARGET" ]; then touch "$LOG_TARGET"; fi

# --- Helper Functions ---
check_root() {
  if [ "$(id -u)" -ne 0 ]; then
    print_error "This script must be run as root. Please use sudo."
    exit 1
  fi
}

backup_file() {
  local file="$1"
  if [ -f "$file" ]; then
    local backup="${file}.bak-$(date +%Y%m%d-%H%M%S)"
    cp "$file" "$backup"
    print_success "Backup created: $backup"
  fi
}

backup_dir() {
  local dir="$1"
  if [ -d "$dir" ]; then
    local backup="${dir}.bak-$(date +%Y%m%d-%H%M%S)"
    cp -a "$dir" "$backup"
    print_success "Backup of directory created: $backup"
  fi
}

apt_update_upgrade() {
  print_info "Updating package lists and upgrading packages..."
  if apt-get update > /dev/null && apt-get upgrade -y > /dev/null; then
    print_success "System packages updated and upgraded."
  else
    print_error "Failed to update/upgrade system packages."
    exit 1
  fi
}

install_dependencies() {
  print_info "Installing necessary dependencies (git, curl, sudo, gpg, rsync)..."
  if apt-get install -y git curl sudo gpg rsync > /dev/null; then
    print_success "Dependencies installed."
  else
    print_error "Failed to install dependencies."
    exit 1
  fi
}

setup_node() {
  print_info "Setting up Node.js repository (NodeSource)..."
  if ! command -v curl &> /dev/null; then
    print_error "curl is required but not found. Please install it first."
    exit 1
  fi
  KEYRING_DIR="/usr/share/keyrings"
  KEYRING_FILE="$KEYRING_DIR/nodesource.gpg"
  if [ ! -d "$KEYRING_DIR" ]; then
    mkdir -p "$KEYRING_DIR" || { print_error "Failed to create $KEYRING_DIR"; exit 1; }
  fi
  print_info "Downloading/refreshing NodeSource GPG key..."
  curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --yes --dearmor -o "$KEYRING_FILE"
  if [ $? -ne 0 ]; then
    print_error "Failed to download or process NodeSource GPG key."
    rm -f "$KEYRING_FILE"
    exit 1
  fi
  echo "deb [signed-by=$KEYRING_FILE] https://deb.nodesource.com/node_$NODE_MAJOR_VERSION.x nodistro main" | tee /etc/apt/sources.list.d/nodesource.list > /dev/null
  if [ $? -ne 0 ]; then
    print_error "Failed to add NodeSource repository to sources list."
    exit 1
  fi
  print_info "Updating package list after adding NodeSource repository..."
  if ! apt-get update > /dev/null; then
    print_error "Failed to update package list after adding NodeSource repository."
    rm -f /etc/apt/sources.list.d/nodesource.list "$KEYRING_FILE"
    exit 1
  fi
  print_info "Installing Node.js ${NODE_MAJOR_VERSION}.x..."
  if apt-get install nodejs -y > /dev/null; then
    print_success "Node.js ${NODE_MAJOR_VERSION}.x installed successfully."
    print_info "Node version: $(node -v)"
    print_info "npm version: $(npm -v)"
  else
    print_error "Failed to install Node.js."
    rm -f /etc/apt/sources.list.d/nodesource.list "$KEYRING_FILE"
    exit 1
  fi
}

create_pulse_user() {
  print_info "Creating dedicated user '$PULSE_USER'..."
  if id "$PULSE_USER" &>/dev/null; then
    print_warning "User '$PULSE_USER' already exists. Skipping creation."
  else
    useradd -r -s /bin/false "$PULSE_USER"
    if [ $? -eq 0 ]; then
      print_success "User '$PULSE_USER' created successfully."
    else
      print_error "Failed to create user '$PULSE_USER'."
      exit 1
    fi
  fi
}

# --- Function to perform update (atomic) ---
perform_update() {
  echo "DEBUG: Entered perform_update function."
  print_info "Attempting to update Pulse..."
  backup_dir "$PULSE_DIR"
  local tmp_dir
  tmp_dir=$(mktemp -d)
  print_info "Cloning latest Pulse to temp directory..."
  if git clone https://github.com/rcourtman/Pulse.git "$tmp_dir" > /dev/null 2>&1; then
    print_success "Repository cloned to $tmp_dir"
  else
    print_error "Failed to clone repository for update."
    rm -rf "$tmp_dir"
    return 1
  fi
  print_info "Installing npm dependencies in temp directory..."
  (cd "$tmp_dir" && npm install --omit=dev --unsafe-perm > /dev/null 2>&1)
  (cd "$tmp_dir/server" && npm install --omit=dev --unsafe-perm > /dev/null 2>&1)
  print_info "Swapping updated directory into place..."
  rsync -a --delete "$tmp_dir/" "$PULSE_DIR/"
  rm -rf "$tmp_dir"
  set_permissions
  echo "DEBUG: Just before calling setup_systemd_service in perform_update."
  print_info "Setting up/refreshing Pulse service ($SERVICE_NAME)..."
  if setup_systemd_service; then
      echo "DEBUG: setup_systemd_service returned success."
      print_success "Pulse service configured and started."
  else
      echo "DEBUG: setup_systemd_service returned failure."
      print_error "Failed to configure or start Pulse service after update."
      return 1 # Propagate the error if setup failed
  fi
  print_success "Pulse updated successfully!"
  echo "DEBUG: Exiting perform_update successfully."
  return 0
}

# --- Function to perform removal ---
perform_remove() {
  print_warning "This will stop and disable the Pulse service and remove the installation directory ($PULSE_DIR)."
  if ! prompt_yes_no "Are you sure you want to remove Pulse?" "N"; then
    print_info "Removal cancelled."
    return 1
  fi
  print_info "Stopping Pulse service ($SERVICE_NAME)..."
  systemctl stop "$SERVICE_NAME" > /dev/null 2>&1
  print_info "Disabling Pulse service ($SERVICE_NAME)..."
  systemctl disable "$SERVICE_NAME" > /dev/null 2>&1
  local service_file_path="/etc/systemd/system/$SERVICE_NAME"
  print_info "Removing systemd service file ($service_file_path)..."
  if [ -f "$service_file_path" ]; then
    rm -f "$service_file_path"
    if [ $? -eq 0 ]; then
      print_success "Service file removed."
      systemctl daemon-reload
    else
      print_warning "Failed to remove service file $service_file_path. Please remove it manually."
    fi
  else
    print_warning "Service file $service_file_path not found."
  fi
  print_info "Removing Pulse installation directory ($PULSE_DIR)..."
  if rm -rf "$PULSE_DIR"; then
    print_success "Installation directory removed."
  else
    print_error "Failed to remove installation directory $PULSE_DIR. Please remove it manually."
    return 1
  fi
  print_success "Pulse removed successfully."
  return 0
}

clone_repository() {
  print_info "Checking Pulse installation directory $PULSE_DIR..."
  if [ -d "$PULSE_DIR" ]; then
    if [ -d "$PULSE_DIR/.git" ]; then
      if [ "$MODE_UPDATE" = true ]; then
        print_info "Running in non-interactive update mode..."
        if perform_update; then
          print_success "Non-interactive update completed successfully."
          exit 0
        else
          print_error "Non-interactive update failed."
          exit 1
        fi
      fi
      print_warning "Pulse seems to be already installed in $PULSE_DIR."
      echo "Choose an action:"
      echo "  1) Update Pulse to the latest version"
      echo "  2) Remove Pulse"
      echo "  3) Cancel installation"
      read -p "Enter your choice (1-3): " user_choice
      case $user_choice in
        1) 
          if perform_update; then
            print_success "Interactive update completed. Continuing script..."
          else
            print_error "Interactive update failed."
            exit 1
          fi
          ;;
        2)
          if perform_remove; then
            exit 0
          else
            exit 1
          fi
          ;;
        3)
          print_info "Installation cancelled by user."
          exit 0
          ;;
        *)
          print_error "Invalid choice. Exiting."
          exit 1
          ;;
      esac
    else
      print_error "Directory $PULSE_DIR exists but does not appear to be a valid Pulse git repository."
      print_error "Please remove this directory manually or choose a different installation path and re-run the script."
      exit 1
    fi
  else
    print_info "Cloning Pulse repository into $PULSE_DIR..."
    if git clone https://github.com/rcourtman/Pulse.git "$PULSE_DIR" > /dev/null 2>&1; then
      print_success "Repository cloned successfully."
    else
      print_error "Failed to clone repository."
      exit 1
    fi
  fi
}

install_npm_deps() {
  print_info "Installing npm dependencies..."
  if [ ! -d "$PULSE_DIR" ]; then
    print_error "Pulse directory $PULSE_DIR not found. Cannot install dependencies."
    exit 1
  fi
  cd "$PULSE_DIR" || { print_error "Failed to change directory to $PULSE_DIR"; exit 1; }
  print_info "Installing root dependencies..."
  if npm install --omit=dev --unsafe-perm > /dev/null 2>&1; then
    print_success "Root dependencies installed."
  else
    print_error "Failed to install root npm dependencies."
    exit 1
  fi
  print_info "Installing server dependencies..."
  cd server || { print_error "Failed to change directory to $PULSE_DIR/server"; exit 1; }
  if npm install --omit=dev --unsafe-perm > /dev/null 2>&1; then
    print_success "Server dependencies installed."
  else
    print_error "Failed to install server npm dependencies."
    exit 1
  fi
  cd ..
}

set_permissions() {
  print_info "Setting permissions for $PULSE_DIR..."
  if chown -R "$PULSE_USER":"$PULSE_USER" "$PULSE_DIR"; then
    print_success "Permissions set correctly."
  else
    print_error "Failed to set permissions for $PULSE_DIR."
    print_warning "Check ownership and permissions for $PULSE_DIR."
  fi
}

configure_environment() {
  print_info "Configuring Pulse environment..."
  local env_example_path="$PULSE_DIR/server/.env.example"
  local env_path="$PULSE_DIR/server/.env"
  if [ ! -f "$env_example_path" ]; then
    print_error "Environment example file not found at $env_example_path. Cannot configure."
    exit 1
  fi
  if [ -f "$env_path" ]; then
    print_info "A configuration file ($env_path) already exists."
    print_info "You can:"
    print_info "  - Keep your current settings (recommended for most users)"
    print_info "  - Overwrite with a fresh template (advanced: erases your current settings)"
    if ! prompt_yes_no "Do you want to overwrite your existing .env with a fresh template? (This will ERASE your current settings!)" "N"; then
      print_info "Keeping your existing .env. No changes made."
      return 0
    fi
    if prompt_yes_no "Do you want to back up your existing .env before overwriting?" "Y"; then
      backup_file "$env_path"
    fi
    if command -v diff &> /dev/null; then
      print_info "----- DIFF: Your .env vs Template (.env.example) -----"
      diff -u "$env_path" "$env_example_path" || true
      print_info "-----------------------------------------------------"
    fi
    print_info "Proceeding to overwrite existing configuration..."
  fi
  # Gather Proxmox Details
  print_info "Please provide your Proxmox connection details:"
  prompt_value " -> Proxmox Host URL (e.g., https://192.168.1.100:8006): " proxmox_host "${PROXMOX_HOST:-}"
  while [ -z "$proxmox_host" ]; do
    print_warning "Proxmox Host URL cannot be empty."
    prompt_value " -> Proxmox Host URL (e.g., https://192.168.1.100:8006): " proxmox_host ""
  done
  echo ""
  print_info "You need a Proxmox API Token. You can create one via the Proxmox Web UI,"
  print_info "or run the following commands on your Proxmox host shell:"
  echo "----------------------------------------------------------------------"
  echo '  # 1. Create user 'pulse-monitor' (enter password when prompted):'
  echo "  pveum useradd pulse-monitor@pam -comment \"API user for Pulse monitoring\""
  echo '  '
  echo '  # 2. Create API token 'pulse' for user (COPY THE SECRET VALUE!):'
  echo "  pveum user token add pulse-monitor@pam pulse --privsep=1"
  echo '  '
  echo '  # 3. Assign PVEAuditor role to user:'
  echo "  pveum acl modify / -user pulse-monitor@pam -role PVEAuditor"
  echo "----------------------------------------------------------------------"
  echo "After running the 'token add' command, copy the Token ID and Secret Value"
  echo "and paste them below."
  echo ""
  prompt_value " -> Proxmox API Token ID (e.g., user@pam!tokenid): " proxmox_token_id "${PROXMOX_TOKEN_ID:-}"
  while [ -z "$proxmox_token_id" ]; do
    print_warning "Proxmox Token ID cannot be empty."
    prompt_value " -> Proxmox API Token ID (e.g., user@pam!tokenid): " proxmox_token_id ""
  done
  prompt_value " -> Proxmox API Token Secret: " proxmox_token_secret "${PROXMOX_TOKEN_SECRET:-}" true
  while [ -z "$proxmox_token_secret" ]; do
    print_warning "Proxmox Token Secret cannot be empty."
    prompt_value " -> Proxmox API Token Secret: " proxmox_token_secret "" true
  done
  prompt_value "Allow self-signed certificates for Proxmox? (y/N): " allow_self_signed "${PROXMOX_ALLOW_SELF_SIGNED_CERTS:-N}"
  local self_signed_value="false"
  if [[ "$allow_self_signed" =~ ^[Yy]$ ]]; then
    self_signed_value="true"
  fi
  prompt_value "Port for Pulse server (leave blank for default 7655): " pulse_port "${PORT:-7655}"
  local port_value="7655"
  if [ -n "$pulse_port" ]; then
    if [[ "$pulse_port" =~ ^[0-9]+$ ]] && [ "$pulse_port" -ge 1 ] && [ "$pulse_port" -le 65535 ]; then
      port_value="$pulse_port"
    else
      print_warning "Invalid port number entered. Using default 7655."
    fi
  fi
  print_info "Creating $env_path from example..."
  if cp "$env_example_path" "$env_path"; then
    sed -i "s|^PROXMOX_HOST=.*|PROXMOX_HOST=$proxmox_host|" "$env_path"
    sed -i "s|^PROXMOX_TOKEN_ID=.*|PROXMOX_TOKEN_ID=$proxmox_token_id|" "$env_path"
    sed -i "s|^PROXMOX_TOKEN_SECRET=.*|PROXMOX_TOKEN_SECRET=$proxmox_token_secret|" "$env_path"
    sed -i "s|^PROXMOX_ALLOW_SELF_SIGNED_CERTS=.*|PROXMOX_ALLOW_SELF_SIGNED_CERTS=$self_signed_value|" "$env_path"
    sed -i "s|^PORT=.*|PORT=$port_value|" "$env_path"
    chown "$PULSE_USER":"$PULSE_USER" "$env_path"
    chmod 600 "$env_path"
    print_success "Environment configured successfully in $env_path."
  else
    print_error "Failed to copy $env_example_path to $env_path."
    exit 1
  fi
}

setup_systemd_service() {
  print_info "Setting up systemd service ($SERVICE_NAME)..."
  local service_file="/etc/systemd/system/$SERVICE_NAME"
  local node_path
  node_path=$(command -v node)
  if [ -z "$node_path" ]; then
    print_error "Could not find Node.js executable path. Cannot create service."
    exit 1
  fi
  local npm_path
  npm_path=$(command -v npm)
  if [ -z "$npm_path" ]; then
    print_warning "Could not find npm executable path. Service might require adjustment."
    local node_dir
    node_dir=$(dirname "$node_path")
    npm_path="$node_dir/npm"
    if ! command -v "$npm_path" &> /dev/null; then
      print_error "Could not reliably find npm executable path. Cannot create service."
      exit 1
    fi
    print_info "Found npm at $npm_path"
  fi
  print_info "Creating service file at $service_file..."
  cat << EOF > "$service_file"
[Unit]
Description=Pulse Monitoring Application
After=network.target

[Service]
Type=simple
User=$PULSE_USER
Group=$PULSE_USER
WorkingDirectory=$PULSE_DIR
ExecStart=$node_path $npm_path run start
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
  if [ $? -ne 0 ]; then
    print_error "Failed to create systemd service file $service_file."
    exit 1
  fi
  chmod 644 "$service_file"
  print_info "Reloading systemd daemon..."
  systemctl daemon-reload
  print_info "Enabling $SERVICE_NAME to start on boot..."
  if systemctl enable "$SERVICE_NAME" > /dev/null 2>&1; then
    print_success "Service enabled successfully."
  else
    print_error "Failed to enable systemd service."
    exit 1
  fi
  print_info "Starting $SERVICE_NAME..."
  if systemctl start "$SERVICE_NAME"; then
    print_success "Service started successfully."
  else
    print_error "Failed to start systemd service."
    print_warning "Please check the service status using: systemctl status $SERVICE_NAME"
    print_warning "And check the logs using: journalctl -u $SERVICE_NAME"
  fi
}

# --- Function to remove the automatic update cron job ---
remove_cron_update() {
  print_info "Removing existing automatic update cron job..."
  (crontab -l -u root 2>/dev/null | grep -v -e "$CRON_IDENTIFIER" -e ".*$SCRIPT_NAME --update.*") | crontab -u root -
  if [ $? -ne 0 ]; then
    print_error "Failed to remove existing cron job. You may need to remove it manually (crontab -e -u root)."
    return 1
  fi
  print_success "Automatic update cron job removed."
  return 0
}

# --- Function to set up automatic updates via cron ---
setup_cron_update() {
  local cron_schedule=""
  local cron_command=""
  local script_path="$SCRIPT_ABS_PATH"
  if [ -z "$script_path" ] || [ ! -f "$script_path" ]; then
    print_warning "Could not reliably determine script path for cron job. Skipping auto-update setup."
    return 1
  fi
  print_info "Choose update frequency:"
  echo "  1) Daily"
  echo "  2) Weekly"
  echo "  3) Monthly"
  read -p "Enter your choice (1-3): " freq_choice
  case $freq_choice in
    1) cron_schedule="@daily" ;;
    2) cron_schedule="@weekly" ;;
    3) cron_schedule="@monthly" ;;
    *) print_error "Invalid choice. Aborting auto-update setup."; return 1 ;;
  esac
  cron_command="$cron_schedule /usr/bin/bash $script_path --update >> $LOG_FILE 2>&1"
  print_info "Removing any previous Pulse cron schedule..."
  (crontab -l -u root 2>/dev/null | grep -v -e "$CRON_IDENTIFIER" -e ".*$SCRIPT_NAME --update.*") | crontab -u root -
  if [ $? -ne 0 ]; then
    print_warning "Could not remove existing cron job (maybe none existed). Proceeding..."
  fi
  print_info "Adding cron job with schedule: $cron_schedule"
  (crontab -l -u root 2>/dev/null; echo "$CRON_IDENTIFIER"; echo "$cron_command") | crontab -u root -
  if [ $? -eq 0 ]; then
    print_success "Cron job for automatic updates configured successfully."
    print_info "Update logs will be written to $LOG_FILE"
    touch "$LOG_FILE" || print_warning "Could not touch log file $LOG_FILE"
    chmod 644 "$LOG_FILE" || print_warning "Could not chmod log file $LOG_FILE"
  else
    print_error "Failed to add cron job. Please check crontab configuration."
    return 1
  fi
  return 0
}

# --- Function to prompt user about setting up cron ---
prompt_for_cron_setup() {
  if [ "$MODE_UPDATE" = true ]; then
    return 0
  fi
  echo ""
  local existing_cron comment_line cron_command_line schedule_word
  # Always get the full crontab, even if empty
  local crontab_content
  crontab_content=$(crontab -l -u root 2>/dev/null || true)
  existing_cron=$(echo "$crontab_content" | grep -A 1 "$CRON_IDENTIFIER" || true)
  if [ -n "$existing_cron" ]; then
    comment_line=$(echo "$existing_cron" | head -n 1)
    cron_command_line=$(echo "$existing_cron" | tail -n 1)
    print_info "Automatic updates are currently configured with this cron schedule:"
    echo "    $comment_line"
    echo "    $cron_command_line"
    schedule_word=$(echo "$cron_command_line" | awk '{print $1}')
    case "$schedule_word" in
      "@daily") print_info "This means: Daily" ;;
      "@weekly") print_info "This means: Weekly" ;;
      "@monthly") print_info "This means: Monthly" ;;
      *) print_info "(Custom cron schedule: $schedule_word)" ;;
    esac
    echo
    read -p "Do you want to change the schedule, remove it, or keep it? (change/remove/keep): " change_cron_confirm
    case "$change_cron_confirm" in
      [Cc]|[Cc]hange)
        setup_cron_update
        ;;
      [Rr]|[Rr]emove)
        remove_cron_update
        ;;
      [Kk]|[Kk]eep)
        print_info "Keeping existing automatic update schedule."
        ;;
      *)
        print_info "Invalid choice. Keeping existing automatic update schedule."
        ;;
    esac
  else
    # Always prompt if no job is found, even if crontab is empty
    if prompt_yes_no "Do you want to set up automatic updates for Pulse?" "N"; then
      setup_cron_update
    else
      print_info "Skipping automatic update setup."
    fi
  fi
}

# --- Health Check ---
health_check() {
  local port_value
  local env_path="$PULSE_DIR/server/.env"
  if [ -f "$env_path" ] && grep -q '^PORT=' "$env_path"; then
    port_value=$(grep '^PORT=' "$env_path" | cut -d'=' -f2)
  else
    port_value="7655"
  fi
  local ip_address
  ip_address=$(hostname -I | awk '{print $1}')
  print_info "Performing health check on http://$ip_address:$port_value ..."
  if command -v curl &> /dev/null; then
    if curl -s --max-time 5 "http://$ip_address:$port_value" | grep -q Pulse; then
      print_success "Pulse dashboard is responding on http://$ip_address:$port_value"
    else
      print_warning "Pulse dashboard did not respond as expected. Please check the service and logs."
    fi
  else
    print_warning "curl not available, skipping HTTP health check."
  fi
}

final_instructions() {
  local ip_address
  ip_address=$(hostname -I | awk '{print $1}')
  local port_value
  local env_path="$PULSE_DIR/server/.env"
  if [ -f "$env_path" ] && grep -q '^PORT=' "$env_path"; then
    port_value=$(grep '^PORT=' "$env_path" | cut -d'=' -f2)
  else
    port_value="7655"
  fi
  echo ""
  print_success "Pulse installation and setup complete!"
  echo "-------------------------------------------------------------"
  print_info "You should be able to access the Pulse dashboard at:"
  if [ -n "$ip_address" ]; then
    echo "  http://$ip_address:$port_value"
  else
    echo "  http://<YOUR-LXC-IP>:$port_value"
    print_warning "Could not automatically determine the LXC IP address."
  fi
  echo ""
  print_info "The Pulse service ($SERVICE_NAME) is running and enabled on boot."
  print_info "To check the status: sudo systemctl status $SERVICE_NAME"
  print_info "To view logs: sudo journalctl -u $SERVICE_NAME -f"
  print_info "Configuration file: $PULSE_DIR/server/.env"
  echo "-------------------------------------------------------------"
}

# --- Main Execution ---
check_root

if command -v readlink &> /dev/null && readlink -f "$0" &> /dev/null; then
  SCRIPT_ABS_PATH=$(readlink -f "$0")
else
  if [[ "$0" == /* ]]; then
    SCRIPT_ABS_PATH="$0"
  else
    SCRIPT_ABS_PATH="$(pwd)/$0"
  fi
  if [ ! -f "$SCRIPT_ABS_PATH" ]; then
    print_warning "Warning: Could not reliably determine absolute script path using fallback."
    SCRIPT_ABS_PATH=""
  fi
fi

apt_update_upgrade
install_dependencies
setup_node
create_pulse_user
clone_repository
install_npm_deps
set_permissions
configure_environment
setup_systemd_service
health_check
final_instructions

echo "DEBUG: About to prompt for cron setup"
prompt_for_cron_setup

print_info "Script finished." 