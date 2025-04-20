#!/bin/bash

# Pulse for Proxmox VE - LXC Creation and Installation Script
# This script runs on the Proxmox VE host to create an LXC container
# and then install Pulse inside it.

# --- Helper Functions (Copied from install-pulse.sh for consistency) ---
print_info() {
  echo -e "\033[1;34m[INFO]\033[0m $1"
}

print_success() {
  echo -e "\033[1;32m[SUCCESS]\033[0m $1"
}

print_warning() {
  echo -e "\033[1;33m[WARNING]\033[0m $1"
}

print_error() {
  echo -e "\033[1;31m[ERROR]\033[0m $1" >&2
}

check_root() {
  if [ "$(id -u)" -ne 0 ]; then
    print_error "This script must be run as root on the Proxmox VE host."
    exit 1
  fi
}

check_pct() {
  if ! command -v pct &> /dev/null; then
    print_error "'pct' command not found. This script must be run on a Proxmox VE host."
    exit 1
  fi
}

# --- Configuration Gathering ---
gather_lxc_config() {
    print_info "Gathering LXC configuration details..."

    # --- Container ID ---
    local default_id
    default_id=$(pvesh get /cluster/nextid)
    read -p "Enter Container ID [$default_id]: " CT_ID
    CT_ID=${CT_ID:-$default_id}
    # Validate ID is a number and not already in use
    while ! [[ "$CT_ID" =~ ^[0-9]+$ ]] || pct status "$CT_ID" > /dev/null 2>&1; do
        if ! [[ "$CT_ID" =~ ^[0-9]+$ ]]; then
            print_warning "Container ID must be a number."
        else
            print_warning "Container ID $CT_ID is already in use."
        fi
        read -p "Enter a unique Container ID: " CT_ID
        [ -z "$CT_ID" ] && { print_error "Container ID cannot be empty."; exit 1; }
    done
    print_info "Using Container ID: $CT_ID"

    # --- Hostname ---
    local default_hostname="pulse-monitor"
    read -p "Enter Hostname [$default_hostname]: " CT_HOSTNAME
    CT_HOSTNAME=${CT_HOSTNAME:-$default_hostname}
    # Basic validation (optional)
    print_info "Using Hostname: $CT_HOSTNAME"

    # --- Password ---
    print_info "Set a root password for the container."
    while true; do
        read -sp "Enter password: " CT_PASSWORD
        echo
        read -sp "Confirm password: " CT_PASSWORD_CONFIRM
        echo
        if [ "$CT_PASSWORD" == "$CT_PASSWORD_CONFIRM" ]; then
            if [ -z "$CT_PASSWORD" ]; then
                 print_warning "Password cannot be empty."
            else
                break
            fi
        else
            print_warning "Passwords do not match. Please try again."
        fi
    done
    # No need to store it globally if only used once for pct create
    print_info "Password set."

    # --- Storage ---
    print_info "Attempting to list suitable storage locations..."
    local storage_options
    # Try using pvesh get storage + jq (requires jq to be installed)
    if command -v jq &> /dev/null; then
        # Get storage IDs where content includes rootdir or images
        storage_options=$(pvesh get /storage --output-format json | jq -r '.[] | select(.content | contains("rootdir") or contains("images")) | .storage')
    else
        print_warning "'jq' command not found. Attempting fallback method for storage detection."
        print_warning "Please install jq (`apt update && apt install jq`) for more reliable storage detection."
        # Fallback using grep/awk on json output (less robust)
        storage_options=$(pvesh get /storage --output-format json | grep -B 1 -E '("content":.*"rootdir"|\"content\":.*"images")' | grep '"storage":' | awk -F'"' '{print $4}')
    fi

    # Check if any storage found
    if [ -z "$storage_options" ]; then
        print_error "No suitable storage locations found for LXC containers (content type 'rootdir' or 'images')."
        print_error "Please configure storage in Proxmox first."
        exit 1
    fi
    PS3="Select storage for root disk: "
    select storage in $storage_options; do
        if [[ -n $storage ]]; then
            CT_STORAGE=$storage
            break
        else
            print_warning "Invalid selection. Please choose a number from the list."
        fi
    done
    print_info "Using Storage: $CT_STORAGE"

    # --- OS Template ---
    print_info "Available OS Templates (on $CT_STORAGE):"
    # Ensure we look for templates on the selected storage
    local template_options
    template_options=$(pvesm list "$CT_STORAGE" --content vztmpl | awk 'NR>1 {print $1}')
    if [ -z "$template_options" ]; then
        print_error "No OS templates found on storage '$CT_STORAGE'."
        print_error "Please download an LXC template first (e.g., Debian or Ubuntu)."
        exit 1
    fi
    PS3="Select OS template: "
    select template in $template_options; do
         if [[ -n $template ]]; then
            # Format template name for pct create (storage:vztmpl/template.tar.gz)
            CT_OSTMPL="$CT_STORAGE:vztmpl/$template"
            break
        else
            print_warning "Invalid selection. Please choose a number from the list."
        fi
    done
     print_info "Using OS Template: $CT_OSTMPL"

    # --- Disk Size ---
    local default_disk="8" # GB
    read -p "Enter root disk size (GB) [$default_disk]: " CT_DISK_SIZE
    CT_DISK_SIZE=${CT_DISK_SIZE:-$default_disk}
    # Validation (optional)
    print_info "Using Disk Size: ${CT_DISK_SIZE}G"

    # --- CPU Cores ---
    local default_cores="1"
    read -p "Enter number of CPU cores [$default_cores]: " CT_CORES
    CT_CORES=${CT_CORES:-$default_cores}
    # Validation (optional)
    print_info "Using CPU Cores: $CT_CORES"

    # --- Memory (RAM) ---
    local default_ram="512" # MB
    read -p "Enter RAM (MB) [$default_ram]: " CT_RAM
    CT_RAM=${CT_RAM:-$default_ram}
    # Validation (optional)
    print_info "Using RAM: ${CT_RAM}MB"

    # --- Network (Using DHCP for simplicity first) ---
    print_info "Network will be configured using DHCP on vmbr0."
    # Later, add options for static IP, different bridge, VLAN etc.
    CT_NET_CONF="name=eth0,bridge=vmbr0,ip=dhcp"

    print_success "LXC Configuration gathered."
}

create_lxc() {
    print_info "Creating LXC container $CT_ID ($CT_HOSTNAME)..."
    # Note: Password needs to be passed securely. pct uses --password.
    # Adding --unprivileged=1 for better security, requires user config on host usually.
    # Using --features nesting=1 if Docker might be run inside later (not strictly needed for Pulse)
    if pct create "$CT_ID" "$CT_OSTMPL" \
        --hostname "$CT_HOSTNAME" \
        --storage "$CT_STORAGE" \
        --rootfs "${CT_STORAGE}:${CT_DISK_SIZE}" \
        --cores "$CT_CORES" \
        --memory "$CT_RAM" \
        --net0 "$CT_NET_CONF" \
        --password "$CT_PASSWORD" \
        --onboot 1 \
        --start 0; then # Create but don't start immediately
        print_success "LXC container $CT_ID created successfully."
    else
        print_error "Failed to create LXC container $CT_ID."
        # Consider cleanup? pct destroy $CT_ID?
        exit 1
    fi
}

start_lxc() {
    print_info "Starting LXC container $CT_ID..."
    if pct start "$CT_ID"; then
        print_success "LXC container $CT_ID started."
    else
        print_error "Failed to start LXC container $CT_ID."
        print_warning "Please check the Proxmox task logs for details."
        exit 1
    fi
}

wait_for_lxc_boot() {
    print_info "Waiting for LXC container $CT_ID to boot and acquire network (up to 2 minutes)..."
    local max_wait=120 # seconds
    local interval=5  # seconds
    local elapsed=0
    local ip_found=0

    while [ $elapsed -lt $max_wait ]; do
        # Check status
        local status
        status=$(pct status "$CT_ID" --quiet)
        if [[ "$status" != "running" ]]; then
            print_warning "LXC $CT_ID is not running (status: $status). Waiting..."
            sleep $interval
            elapsed=$((elapsed + interval))
            continue
        fi

        # Check for network connectivity (attempt to get IP)
        # Use pct exec with a timeout for the internal command
        if pct exec "$CT_ID" -- timeout 10 ip -4 addr show scope global | grep -q 'inet'; then
            print_success "LXC container $CT_ID is running and network is up."
            ip_found=1
            break
        else
            print_info "LXC $CT_ID is running, but network not fully up yet. Waiting..."
        fi

        sleep $interval
        elapsed=$((elapsed + interval))
    done

    if [ $ip_found -eq 0 ]; then
        print_error "LXC container $CT_ID did not boot or acquire an IP address within $max_wait seconds."
        print_warning "Please check the container console and network configuration."
        # Optionally offer to stop/destroy?
        exit 1
    fi
}

run_install_script_in_lxc() {
    print_info "Running Pulse installation script inside LXC container $CT_ID..."
    print_info "This may take several minutes depending on network speed and system performance."

    local install_cmd="bash -c \"\$(wget -qLO - https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-pulse.sh)\" --"

    # Execute the command using pct exec
    # We need to run this as root inside the container
    # The '--' separates pct options from the command to be executed
    if pct exec "$CT_ID" -- bash -c "$install_cmd"; then
        print_success "Pulse installation script executed successfully inside LXC $CT_ID."
        # The internal script (install-pulse.sh) should print the final access URL.
        # We can try to fetch the IP again here as a convenience.
        local final_ip
        final_ip=$(pct exec "$CT_ID" -- timeout 5 ip -4 addr show scope global | awk '/inet / {print $2}' | cut -d/ -f1 | head -n1)
        if [ -n "$final_ip" ]; then
            # Attempt to read port from install script defaults/config if possible?
            # Hardcoding 7655 for now as it's the default.
             print_info "Pulse should be accessible at: http://$final_ip:7655 (assuming default port)"
        else
            print_warning "Could not retrieve final IP address from container. Please check manually."
        fi
    else
        print_error "Failed to execute Pulse installation script inside LXC $CT_ID."
        print_warning "Check the output above for errors from the installation script."
        print_warning "You may need to access the container console (\`pct enter $CT_ID\`) to troubleshoot."
        exit 1
    fi
}

# --- Main Execution ---
check_root
check_pct

gather_lxc_config

create_lxc # Create the container
start_lxc  # Start the container
wait_for_lxc_boot # Wait for it to be ready
run_install_script_in_lxc # Run the install script inside

print_success "Pulse LXC setup script completed." 