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
    local storage_options_display=() # Array for select prompt display
    local storage_ids=() # Array for actual storage IDs
    local storage_template_support=() # Array to track template support (true/false)

    # Use jq if available for reliable parsing
    if command -v jq &> /dev/null; then
        # Process each storage entry from pvesh output
        while IFS=$'\t' read -r id content; do
            # Check if it supports disks (rootdir or images)
            if [[ $content == *\"rootdir\"* || $content == *\"images\"* ]]; then
                local supports_templates=\"false\"
                local display_suffix=\"Disk Only\"
                # Check if it ALSO supports templates (vztmpl)
                if [[ $content == *\"vztmpl\"* ]]; then
                    supports_templates=\"true\"
                    display_suffix=\"Templates OK\"
                fi
                storage_options_display+=(\"$id $display_suffix\")
                storage_ids+=(\"$id\")
                storage_template_support+=(\"$supports_templates\")
            fi
        done < <(pvesh get /storage --output-format json | jq -r '.[] | [.storage, .content] | @tsv')
    else
        # Fallback without jq (less robust, might fail on complex setups)
        print_warning "'jq' command not found. Storage detection might be less reliable."
        print_warning "Please install jq (`apt update && apt install jq`) for the best experience."
        # Basic grep/awk approach - less accurate for template support
        local disk_storage
        disk_storage=$(pvesh get /storage --output-format json | grep -B 1 -E '(\"content\":.*\"rootdir\"|\"content\":.*\"images\")\' | grep \'\"storage\":\' | awk -F\'\"\' '{print $4}')
        for id in $disk_storage; do
             storage_options_display+=(\"$id Template support unknown\") # Cannot reliably check template support here - Removed parentheses
             storage_ids+=(\"$id\")
             storage_template_support+=(\"unknown\") # Mark as unknown
        done
    fi

    # Check if any suitable storage found
    if [ ${#storage_ids[@]} -eq 0 ]; then
        print_error "No storage locations found suitable for LXC disks (content type 'rootdir' or 'images')."
        print_error "Please configure storage content types in the Proxmox UI first."
        exit 1
    fi

    print_info "Select storage for the container's root disk:"
    PS3="Select storage number: "
    select display_option in "${storage_options_display[@]}"; do
        # $REPLY is the index chosen by the user
        if [[ "$REPLY" =~ ^[0-9]+$ ]] && [ "$REPLY" -ge 1 ] && [ "$REPLY" -le ${#storage_ids[@]} ]; then
            local chosen_index=$((REPLY - 1))
            CT_STORAGE=${storage_ids[$chosen_index]}
            CT_STORAGE_SUPPORTS_TEMPLATES=${storage_template_support[$chosen_index]}
            print_info "Using Storage: $CT_STORAGE (Supports Templates: $CT_STORAGE_SUPPORTS_TEMPLATES)"
            break
        else
            print_warning "Invalid selection. Please choose a number from the list."
        fi
    done

    # --- OS Template ---
    print_info "Checking for OS Templates on storage '$CT_STORAGE'..."
    local template_options
    # Parse standard output of pvesm list (doesn't support json)
    # Expected format: <storage>:<type>/<filename> ...
    # We want just the <filename> part
    template_options=$(pvesm list "$CT_STORAGE" --content vztmpl | awk -v storage="$CT_STORAGE" 'NR>1 {sub(storage ":vztmpl/", "", $1); print $1}')

    if [ -z "$template_options" ]; then
        # No templates found, check if download is possible using the reliable boolean check
        # (Ignore the display suffix which might be from a cached script version)
        if [[ "$CT_STORAGE_SUPPORTS_TEMPLATES" == "false" ]]; then
             print_error "No existing OS templates found on selected storage '$CT_STORAGE',"
             print_error "and this storage location does not support template downloads (missing 'vztmpl' content type)."
             print_error "Please either enable 'vztmpl' content for '$CT_STORAGE' in the Proxmox UI,"
             print_error "or restart the script and choose storage marked as '(Templates OK)'."
             exit 1
        elif [[ "$CT_STORAGE_SUPPORTS_TEMPLATES" == "true" ]]; then # Explicitly check for true
             print_warning "No OS templates found on storage '$CT_STORAGE'. Attempting to download one."
             # Fall through to download logic
        else # Should only be reachable if CT_STORAGE_SUPPORTS_TEMPLATES is still 'unknown'
             # This case was handled above by exiting if unknown
             print_error "Internal logic error determining template support. Exiting." # Safety net
             exit 1
        fi

        # --- Download Logic (Only reached if templates not found AND storage support is true) ---
        print_info "Updating available template list..."
        if ! pveam update > /dev/null; then
            print_error "Failed to update template list. Please check network connectivity or run 'pveam update' manually."
            exit 1
        fi
        print_info "Fetching available templates (this may take a moment)..."
        local downloadable_templates
        # Get system templates, parse filename (usually the second column after 'system')
        downloadable_templates=$(pveam available --section system | awk 'NR>1 {print $2}')

        if [ -z "$downloadable_templates" ]; then
             print_error "Could not retrieve list of downloadable templates."
             exit 1
        fi

        echo "Available templates to download:"
        PS3="Select a template to download to '$CT_STORAGE': "
        select template_to_download in $downloadable_templates; do
            if [[ -n $template_to_download ]]; then
                print_info "Attempting to download '$template_to_download' to '$CT_STORAGE'..."
                # Show download progress
                if pveam download "$CT_STORAGE" "$template_to_download"; then
                    print_success "Template '$template_to_download' downloaded successfully."
                    # Set the template path for container creation
                    CT_OSTMPL="$CT_STORAGE:vztmpl/$template_to_download"
                    break # Exit the select loop
                else
                    print_error "Failed to download template '$template_to_download'."
                    print_warning "Please check disk space on '$CT_STORAGE', network connectivity, and Proxmox task logs."
                    # Optionally loop back or exit? Exiting for now.
                    exit 1
                fi
            else
                print_warning "Invalid selection. Please choose a number from the list."
            fi
        done
        # Check if CT_OSTMPL was set (i.e., download was attempted and potentially successful)
        if [ -z "$CT_OSTMPL" ]; then
             print_error "Template selection or download failed. Exiting."
             exit 1
        fi
    else
        # Templates exist, let user choose
        print_info "Available OS Templates found on '$CT_STORAGE': "
        PS3="Select OS template: "
        select existing_template in $template_options; do
             if [[ -n $existing_template ]]; then
                # Format template name for pct create
                CT_OSTMPL="$CT_STORAGE:vztmpl/$existing_template"
                break
            else
                print_warning "Invalid selection. Please choose a number from the list."
            fi
        done
    fi
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