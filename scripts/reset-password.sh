#!/bin/bash

# Pulse Password Reset Tool
# This script helps reset the Pulse UI password

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if running as root or with sudo
if [ "$EUID" -ne 0 ]; then 
    echo -e "${RED}Please run this script as root or with sudo${NC}"
    exit 1
fi

# Pulse configuration location
ENV_FILE="/opt/pulse/.env"
BACKUP_FILE="/opt/pulse/.env.backup.$(date +%Y%m%d_%H%M%S)"

echo -e "${GREEN}=== Pulse Password Reset Tool ===${NC}"
echo ""

# Check if .env file exists
if [ ! -f "$ENV_FILE" ]; then
    echo -e "${YELLOW}No .env file found. Creating new configuration...${NC}"
    touch "$ENV_FILE"
    chown pulse:pulse "$ENV_FILE" 2>/dev/null || true
    chmod 600 "$ENV_FILE"
fi

# Backup existing configuration
if [ -s "$ENV_FILE" ]; then
    echo -e "${GREEN}Creating backup: $BACKUP_FILE${NC}"
    cp "$ENV_FILE" "$BACKUP_FILE"
fi

# Function to update or add a line in .env
update_env_var() {
    local key="$1"
    local value="$2"
    
    if grep -q "^${key}=" "$ENV_FILE"; then
        # Update existing line
        sed -i "s|^${key}=.*|${key}=${value}|" "$ENV_FILE"
    else
        # Add new line
        echo "${key}=${value}" >> "$ENV_FILE"
    fi
}

# Ask what the user wants to do
echo "What would you like to do?"
echo "1) Reset password (keep existing username)"
echo "2) Set new username and password"
echo "3) Disable authentication completely"
echo "4) Show current authentication status"
echo ""
read -p "Enter choice (1-4): " choice

case $choice in
    1)
        # Reset password only
        echo ""
        # Check if there's an existing username
        if grep -q "^PULSE_AUTH_USER=" "$ENV_FILE"; then
            current_user=$(grep "^PULSE_AUTH_USER=" "$ENV_FILE" | cut -d'=' -f2)
            echo -e "${GREEN}Current username: $current_user${NC}"
        else
            echo -e "${YELLOW}No username found. Please use option 2 to set both username and password.${NC}"
            exit 1
        fi
        
        echo ""
        read -s -p "Enter new password (minimum 8 characters): " password
        echo ""
        read -s -p "Confirm new password: " password2
        echo ""
        
        if [ "$password" != "$password2" ]; then
            echo -e "${RED}Passwords do not match!${NC}"
            exit 1
        fi
        
        if [ ${#password} -lt 8 ]; then
            echo -e "${RED}Password must be at least 8 characters!${NC}"
            exit 1
        fi
        
        # Update password
        update_env_var "PULSE_AUTH_PASS" "$password"
        # Ensure auth is enabled
        sed -i '/^DISABLE_AUTH=/d' "$ENV_FILE"
        
        echo -e "${GREEN}✓ Password reset successfully${NC}"
        ;;
        
    2)
        # Set new username and password
        echo ""
        read -p "Enter new username: " username
        
        if [ -z "$username" ]; then
            echo -e "${RED}Username cannot be empty!${NC}"
            exit 1
        fi
        
        read -s -p "Enter new password (minimum 8 characters): " password
        echo ""
        read -s -p "Confirm new password: " password2
        echo ""
        
        if [ "$password" != "$password2" ]; then
            echo -e "${RED}Passwords do not match!${NC}"
            exit 1
        fi
        
        if [ ${#password} -lt 8 ]; then
            echo -e "${RED}Password must be at least 8 characters!${NC}"
            exit 1
        fi
        
        # Update username and password
        update_env_var "PULSE_AUTH_USER" "$username"
        update_env_var "PULSE_AUTH_PASS" "$password"
        # Ensure auth is enabled
        sed -i '/^DISABLE_AUTH=/d' "$ENV_FILE"
        
        echo -e "${GREEN}✓ Username and password set successfully${NC}"
        ;;
        
    3)
        # Disable authentication
        echo ""
        echo -e "${YELLOW}WARNING: This will disable all authentication for Pulse!${NC}"
        echo -e "${YELLOW}Anyone with network access will be able to view and modify your configuration.${NC}"
        echo ""
        read -p "Are you sure you want to disable authentication? (yes/no): " confirm
        
        if [ "$confirm" = "yes" ]; then
            # Remove auth credentials and set DISABLE_AUTH=true
            sed -i '/^PULSE_AUTH_USER=/d' "$ENV_FILE"
            sed -i '/^PULSE_AUTH_PASS=/d' "$ENV_FILE"
            update_env_var "DISABLE_AUTH" "true"
            
            echo -e "${GREEN}✓ Authentication disabled${NC}"
        else
            echo "Cancelled"
            exit 0
        fi
        ;;
        
    4)
        # Show current status
        echo ""
        echo -e "${GREEN}Current Authentication Status:${NC}"
        echo ""
        
        if grep -q "^DISABLE_AUTH=true" "$ENV_FILE"; then
            echo -e "${YELLOW}Authentication is DISABLED${NC}"
        elif grep -q "^PULSE_AUTH_USER=" "$ENV_FILE"; then
            current_user=$(grep "^PULSE_AUTH_USER=" "$ENV_FILE" | cut -d'=' -f2)
            echo -e "${GREEN}Authentication is ENABLED${NC}"
            echo "Username: $current_user"
            
            if grep -q "^PULSE_AUTH_PASS=" "$ENV_FILE"; then
                pass_value=$(grep "^PULSE_AUTH_PASS=" "$ENV_FILE" | cut -d'=' -f2)
                # Check if it's a bcrypt hash (starts with $2 and is ~60 chars)
                if [[ $pass_value == \$2* ]] && [ ${#pass_value} -ge 55 ]; then
                    echo "Password: [HASHED - Secure]"
                else
                    echo "Password: [SET - Plain text, will be hashed on restart]"
                fi
            else
                echo -e "${YELLOW}Password: [NOT SET]${NC}"
            fi
        else
            echo -e "${YELLOW}No authentication configured${NC}"
            echo "Use option 2 to set up authentication"
        fi
        
        echo ""
        exit 0
        ;;
        
    *)
        echo -e "${RED}Invalid choice${NC}"
        exit 1
        ;;
esac

# Set proper permissions
chown pulse:pulse "$ENV_FILE" 2>/dev/null || true
chmod 600 "$ENV_FILE"

# Ask if user wants to restart Pulse
echo ""
echo -e "${YELLOW}Changes have been saved to $ENV_FILE${NC}"
echo ""
read -p "Would you like to restart Pulse now to apply changes? (y/n): " restart

if [ "$restart" = "y" ] || [ "$restart" = "Y" ]; then
    # Check which service is running
    if systemctl is-active --quiet pulse-backend; then
        echo "Restarting pulse-backend service..."
        systemctl restart pulse-backend
        echo -e "${GREEN}✓ Service restarted${NC}"
    elif systemctl is-active --quiet pulse; then
        echo "Restarting pulse service..."
        systemctl restart pulse
        echo -e "${GREEN}✓ Service restarted${NC}"
    elif systemctl is-active --quiet pulse-dev; then
        echo "Restarting pulse-dev service..."
        systemctl restart pulse-dev
        echo -e "${GREEN}✓ Service restarted${NC}"
    else
        echo -e "${YELLOW}No Pulse service found running. Please restart manually if needed.${NC}"
    fi
else
    echo ""
    echo "To apply changes, restart Pulse manually with:"
    echo "  sudo systemctl restart pulse"
    echo "  or"
    echo "  sudo systemctl restart pulse-backend"
fi

echo ""
echo -e "${GREEN}Done!${NC}"