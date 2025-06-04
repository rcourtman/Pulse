#!/bin/bash

# Setup polkit rule to allow pulse user to restart pulse service without sudo

print_info() {
    echo -e "\033[0;36m➜\033[0m $1"
}

print_success() {
    echo -e "\033[0;32m✓\033[0m $1"
}

print_error() {
    echo -e "\033[0;31m✗\033[0m $1"
}

# Check if running as root
if [[ $EUID -ne 0 ]]; then
    print_error "This script must be run as root (use sudo)"
    exit 1
fi

print_info "Setting up polkit rule for pulse service management..."

# Create polkit rules directory if it doesn't exist
mkdir -p /etc/polkit-1/rules.d

# Create the polkit rule
cat > /etc/polkit-1/rules.d/10-pulse-service.rules << 'EOF'
/* Allow pulse user to manage pulse.service without password */
polkit.addRule(function(action, subject) {
    if (action.id == "org.freedesktop.systemd1.manage-units" &&
        action.lookup("unit") == "pulse.service" &&
        subject.user == "pulse") {
        return polkit.Result.YES;
    }
});
EOF

# Set correct permissions
chmod 644 /etc/polkit-1/rules.d/10-pulse-service.rules

print_success "Polkit rule installed successfully"
print_info "The pulse user can now restart the pulse service without sudo"

# Restart polkit to apply changes
if systemctl is-active --quiet polkit; then
    print_info "Restarting polkit to apply changes..."
    systemctl restart polkit
    print_success "Polkit restarted"
fi

print_success "Setup complete! Updates can now run without sudo."