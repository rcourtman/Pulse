#!/bin/bash

# Make script executable if it isn't already
chmod +x "$0"

# Define colors for better readability
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Display header
echo -e "${BLUE}=======================================${NC}"
echo -e "${BLUE}       Pulse Application Launcher      ${NC}"
echo -e "${BLUE}=======================================${NC}"

# Function to display options with improved clarity for new users
show_options() {
  echo -e "\n${GREEN}Welcome to Pulse!${NC} For beginners, try option ${YELLOW}2${NC} (Mock Data)."
  
  echo -e "\n${GREEN}DEVELOPMENT:${NC}"
  echo -e "${YELLOW}1)${NC} Dev - Real Proxmox ${BLUE}(Uses your Proxmox servers, port 3000)${NC}"
  echo -e "${YELLOW}2)${NC} Dev - Mock Data ${GREEN}★ RECOMMENDED FOR BEGINNERS ★${NC} ${BLUE}(No real servers needed, port 3000)${NC}"
  
  echo -e "\n${GREEN}PRODUCTION:${NC}"
  echo -e "${YELLOW}3)${NC} Production ${BLUE}(Real Proxmox, optimized build, port 7654)${NC}"
  
  echo -e "\n${GREEN}DOCKER:${NC}"
  echo -e "${YELLOW}4)${NC} Docker Dev ${BLUE}(Mock data, hot-reloading, ports 7654/3000)${NC}"
  echo -e "${YELLOW}5)${NC} Docker Prod ${BLUE}(Real Proxmox, optimized build, port 7654)${NC}"
  
  echo -e "\n${YELLOW}q)${NC} Quit"
  echo -e "\n${GREEN}Choice (1-5 or q):${NC} "
}

# Main menu loop
while true; do
  show_options
  read -r choice
  
  case $choice in
    1)
      echo -e "\n${BLUE}Starting development environment with real Proxmox data...${NC}"
      # Ensure .env file exists
      [ -f .env ] || cp .env.example .env
      
      # Configure environment for dev-real (with mock data disabled)
      node scripts/configure-env.js dev-real
      
      # Ensure cluster auto-detection is enabled
      if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS requires an empty string with sed
        sed -i '' 's/PROXMOX_AUTO_DETECT_CLUSTER=false/PROXMOX_AUTO_DETECT_CLUSTER=true/' .env
        sed -i '' 's/PROXMOX_CLUSTER_MODE=true/PROXMOX_CLUSTER_MODE=false/' .env
      else
        # Linux version
        sed -i 's/PROXMOX_AUTO_DETECT_CLUSTER=false/PROXMOX_AUTO_DETECT_CLUSTER=true/' .env
        sed -i 's/PROXMOX_CLUSTER_MODE=true/PROXMOX_CLUSTER_MODE=false/' .env
      fi
      
      echo -e "${GREEN}Configured environment to use real Proxmox data with automatic cluster detection${NC}"
      
      # Set environment variables directly in the current shell and for any child processes
      export USE_MOCK_DATA=false
      export MOCK_DATA_ENABLED=false
      export NODE_ENV=development
      export PROXMOX_AUTO_DETECT_CLUSTER=true
      export PROXMOX_CLUSTER_MODE=false
      
      # Now run with real data - run the start-dev.sh script directly instead of through start.js
      # This ensures our environment variables are passed through properly
      chmod +x ./scripts/start-dev.sh
      ./scripts/start-dev.sh
      break
      ;;
    2)
      echo -e "\n${BLUE}Starting development environment with mock data...${NC}"
      # Ensure .env file exists
      [ -f .env ] || cp .env.example .env
      
      # Configure environment for dev and explicitly set USE_MOCK_DATA=true
      node scripts/configure-env.js dev
      
      # Ensure mock data settings are set to true (should already be, but just to be safe)
      if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS requires an empty string with sed
        sed -i '' 's/USE_MOCK_DATA=false/USE_MOCK_DATA=true/' .env
        sed -i '' 's/MOCK_DATA_ENABLED=false/MOCK_DATA_ENABLED=true/' .env
      else
        # Linux version
        sed -i 's/USE_MOCK_DATA=false/USE_MOCK_DATA=true/' .env
        sed -i 's/MOCK_DATA_ENABLED=false/MOCK_DATA_ENABLED=true/' .env
      fi
      
      echo -e "${GREEN}Configured environment to use mock data${NC}"
      
      # Set environment variables directly in the current shell and for any child processes
      export USE_MOCK_DATA=true
      export MOCK_DATA_ENABLED=true
      export NODE_ENV=development
      
      # Now run with mock data - run the start-dev.sh script directly
      chmod +x ./scripts/start-dev.sh
      ./scripts/start-dev.sh
      break
      ;;
    3)
      echo -e "\n${BLUE}Starting production environment...${NC}"
      # Ensure .env file exists
      [ -f .env ] || cp .env.example .env
      
      # Configure environment for production
      node scripts/configure-env.js prod
      
      # Ensure mock data settings are set to false for production
      if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS requires an empty string with sed
        sed -i '' 's/USE_MOCK_DATA=true/USE_MOCK_DATA=false/' .env
        sed -i '' 's/MOCK_DATA_ENABLED=true/MOCK_DATA_ENABLED=false/' .env
      else
        # Linux version
        sed -i 's/USE_MOCK_DATA=true/USE_MOCK_DATA=false/' .env
        sed -i 's/MOCK_DATA_ENABLED=false/MOCK_DATA_ENABLED=false/' .env
      fi
      
      echo -e "${GREEN}Configured environment for production mode${NC}"
      
      # Set environment variables directly in the current shell and for any child processes
      export USE_MOCK_DATA=false
      export MOCK_DATA_ENABLED=false
      export NODE_ENV=production
      
      # Run production script directly
      chmod +x ./scripts/start-prod.sh
      ./scripts/start-prod.sh
      break
      ;;
    4)
      echo -e "\n${BLUE}Starting Docker development environment...${NC}"
      # Ensure .env file exists but don't overwrite an existing one
      if [ ! -f .env ]; then
        echo -e "${YELLOW}Creating .env file from .env.example${NC}"
        cp .env.example .env
      else
        echo -e "${GREEN}Using existing .env file${NC}"
      fi
      
      # Configure for Docker development
      if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS requires an empty string with sed
        # Set development environment
        sed -i '' 's/NODE_ENV=production/NODE_ENV=development/' .env
        # First ensure there are no duplicated .dev extensions
        sed -i '' 's|DOCKERFILE=docker/Dockerfile.dev.dev|DOCKERFILE=docker/Dockerfile.dev|' .env
        # Use the development Dockerfile (only if not already set to .dev)
        sed -i '' 's|DOCKERFILE=docker/Dockerfile$|DOCKERFILE=docker/Dockerfile.dev|' .env
        # Enable mock data for Docker development
        sed -i '' 's/USE_MOCK_DATA=false/USE_MOCK_DATA=true/' .env
        sed -i '' 's/MOCK_DATA_ENABLED=false/MOCK_DATA_ENABLED=true/' .env
        # Configure Docker development mounts
        sed -i '' 's/# DEV_SRC_MOUNT/DEV_SRC_MOUNT/' .env
        sed -i '' 's/# DEV_FRONTEND_SRC_MOUNT/DEV_FRONTEND_SRC_MOUNT/' .env
        sed -i '' 's/# DEV_FRONTEND_PUBLIC_MOUNT/DEV_FRONTEND_PUBLIC_MOUNT/' .env
        sed -i '' 's/# DEV_FRONTEND_INDEX_MOUNT/DEV_FRONTEND_INDEX_MOUNT/' .env
        sed -i '' 's/# DEV_FRONTEND_CONFIG_MOUNT/DEV_FRONTEND_CONFIG_MOUNT/' .env
        sed -i '' 's/# DEV_SCRIPTS_MOUNT/DEV_SCRIPTS_MOUNT/' .env
        sed -i '' 's/# DEV_ENV_MOUNT/DEV_ENV_MOUNT/' .env
      else
        # Linux version
        # Set development environment
        sed -i 's/NODE_ENV=production/NODE_ENV=development/' .env
        # First ensure there are no duplicated .dev extensions
        sed -i 's|DOCKERFILE=docker/Dockerfile.dev.dev|DOCKERFILE=docker/Dockerfile.dev|' .env
        # Use the development Dockerfile (only if not already set to .dev)
        sed -i 's|DOCKERFILE=docker/Dockerfile$|DOCKERFILE=docker/Dockerfile.dev|' .env
        # Enable mock data for Docker development
        sed -i 's/USE_MOCK_DATA=false/USE_MOCK_DATA=true/' .env
        sed -i 's/MOCK_DATA_ENABLED=false/MOCK_DATA_ENABLED=true/' .env
        # Configure Docker development mounts
        sed -i 's/# DEV_SRC_MOUNT/DEV_SRC_MOUNT/' .env
        sed -i 's/# DEV_FRONTEND_SRC_MOUNT/DEV_FRONTEND_SRC_MOUNT/' .env
        sed -i 's/# DEV_FRONTEND_PUBLIC_MOUNT/DEV_FRONTEND_PUBLIC_MOUNT/' .env
        sed -i 's/# DEV_FRONTEND_INDEX_MOUNT/DEV_FRONTEND_INDEX_MOUNT/' .env
        sed -i 's/# DEV_FRONTEND_CONFIG_MOUNT/DEV_FRONTEND_CONFIG_MOUNT/' .env
        sed -i 's/# DEV_SCRIPTS_MOUNT/DEV_SCRIPTS_MOUNT/' .env
        sed -i 's/# DEV_ENV_MOUNT/DEV_ENV_MOUNT/' .env
      fi
      
      echo -e "${GREEN}Configured Docker environment for development with mock data${NC}"
      docker compose up --build
      break
      ;;
    5)
      echo -e "\n${BLUE}Starting Docker production environment...${NC}"
      # Ensure .env file exists but don't overwrite an existing one
      if [ ! -f .env ]; then
        echo -e "${YELLOW}Creating .env file from .env.example${NC}"
        cp .env.example .env
      else
        echo -e "${GREEN}Using existing .env file${NC}"
      fi
      
      # Define a function to check if Proxmox credentials are valid
      check_proxmox_credentials() {
        # Check if the default placeholder is still in the config
        if grep -q "PROXMOX_NODE_1_TOKEN_SECRET=your-token-secret" .env; then
          return 1  # Invalid credentials
        else
          return 0  # Valid credentials
        fi
      }
      
      # Ask the user if they want to enter Proxmox credentials or use mock data
      if ! check_proxmox_credentials; then
        echo -e "${YELLOW}No valid Proxmox credentials found in configuration.${NC}"
        echo -e "${GREEN}Do you want to:${NC}"
        echo -e "${YELLOW}1)${NC} Enter valid Proxmox credentials"
        echo -e "${YELLOW}2)${NC} Use mock data instead"
        echo -e "\n${GREEN}Enter your choice (1 or 2):${NC} "
        read -r cred_choice
        
        case $cred_choice in
          1)
            echo -e "\n${GREEN}Please enter your Proxmox credentials:${NC}"
            echo -e "${YELLOW}Node name (e.g., pve-1):${NC} "
            read -r proxmox_name
            echo -e "${YELLOW}Host URL (e.g., https://your-proxmox-server:8006):${NC} "
            read -r proxmox_host
            echo -e "${YELLOW}Token ID (e.g., root@pam!token-name):${NC} "
            read -r proxmox_token_id
            echo -e "${YELLOW}Token Secret:${NC} "
            read -r proxmox_token_secret
            
            # Update the .env file with the provided credentials
            if [[ "$OSTYPE" == "darwin"* ]]; then
              # macOS requires an empty string with sed
              sed -i '' "s|PROXMOX_NODE_1_NAME=.*|PROXMOX_NODE_1_NAME=$proxmox_name|" .env
              sed -i '' "s|PROXMOX_NODE_1_HOST=.*|PROXMOX_NODE_1_HOST=$proxmox_host|" .env
              sed -i '' "s|PROXMOX_NODE_1_TOKEN_ID=.*|PROXMOX_NODE_1_TOKEN_ID=$proxmox_token_id|" .env
              sed -i '' "s|PROXMOX_NODE_1_TOKEN_SECRET=.*|PROXMOX_NODE_1_TOKEN_SECRET=$proxmox_token_secret|" .env
              sed -i '' 's/USE_MOCK_DATA=true/USE_MOCK_DATA=false/' .env
              sed -i '' 's/MOCK_DATA_ENABLED=true/MOCK_DATA_ENABLED=false/' .env
            else
              # Linux version
              sed -i "s|PROXMOX_NODE_1_NAME=.*|PROXMOX_NODE_1_NAME=$proxmox_name|" .env
              sed -i "s|PROXMOX_NODE_1_HOST=.*|PROXMOX_NODE_1_HOST=$proxmox_host|" .env
              sed -i "s|PROXMOX_NODE_1_TOKEN_ID=.*|PROXMOX_NODE_1_TOKEN_ID=$proxmox_token_id|" .env
              sed -i "s|PROXMOX_NODE_1_TOKEN_SECRET=.*|PROXMOX_NODE_1_TOKEN_SECRET=$proxmox_token_secret|" .env
              sed -i 's/USE_MOCK_DATA=true/USE_MOCK_DATA=false/' .env
              sed -i 's/MOCK_DATA_ENABLED=true/MOCK_DATA_ENABLED=false/' .env
            fi
            
            echo -e "${GREEN}Configured Docker environment for production with real Proxmox data${NC}"
            ;;
          2)
            # Configure for Docker production with mock data
            if [[ "$OSTYPE" == "darwin"* ]]; then
              # macOS requires an empty string with sed
              # Keep NODE_ENV=production (already set in .env.example)
              # Keep DOCKERFILE=docker/Dockerfile (already set in .env.example)
              # Enable mock data for Docker production
              sed -i '' 's/USE_MOCK_DATA=false/USE_MOCK_DATA=true/' .env
              sed -i '' 's/MOCK_DATA_ENABLED=false/MOCK_DATA_ENABLED=true/' .env
            else
              # Linux version
              # Keep NODE_ENV=production (already set in .env.example)
              # Keep DOCKERFILE=docker/Dockerfile (already set in .env.example)
              # Enable mock data for Docker production
              sed -i 's/USE_MOCK_DATA=false/USE_MOCK_DATA=true/' .env
              sed -i 's/MOCK_DATA_ENABLED=false/MOCK_DATA_ENABLED=true/' .env
            fi
            
            echo -e "${GREEN}Configured Docker environment for production with mock data${NC}"
            ;;
          *)
            echo -e "${YELLOW}Invalid choice. Using mock data as a fallback.${NC}"
            # Configure for Docker production with mock data
            if [[ "$OSTYPE" == "darwin"* ]]; then
              sed -i '' 's/USE_MOCK_DATA=false/USE_MOCK_DATA=true/' .env
              sed -i '' 's/MOCK_DATA_ENABLED=false/MOCK_DATA_ENABLED=true/' .env
            else
              sed -i 's/USE_MOCK_DATA=false/USE_MOCK_DATA=true/' .env
              sed -i 's/MOCK_DATA_ENABLED=false/MOCK_DATA_ENABLED=true/' .env
            fi
            
            echo -e "${GREEN}Configured Docker environment for production with mock data${NC}"
            ;;
        esac
      else
        # Valid credentials already exist
        # Set production environment
        if [[ "$OSTYPE" == "darwin"* ]]; then
          # macOS requires an empty string with sed
          # Ensure NODE_ENV is set to production
          sed -i '' 's/NODE_ENV=development/NODE_ENV=production/' .env
          # Ensure DOCKERFILE is set to production
          sed -i '' 's|DOCKERFILE=docker/Dockerfile.dev|DOCKERFILE=docker/Dockerfile|' .env
          # Disable mock data for Docker production to match regular production
          sed -i '' 's/USE_MOCK_DATA=true/USE_MOCK_DATA=false/' .env
          sed -i '' 's/MOCK_DATA_ENABLED=true/MOCK_DATA_ENABLED=false/' .env
        else
          # Linux version
          # Ensure NODE_ENV is set to production
          sed -i 's/NODE_ENV=development/NODE_ENV=production/' .env
          # Ensure DOCKERFILE is set to production
          sed -i 's|DOCKERFILE=docker/Dockerfile.dev|DOCKERFILE=docker/Dockerfile|' .env
          # Disable mock data for Docker production to match regular production
          sed -i 's/USE_MOCK_DATA=true/USE_MOCK_DATA=false/' .env
          sed -i 's/MOCK_DATA_ENABLED=true/MOCK_DATA_ENABLED=false/' .env
        fi
        
        echo -e "${GREEN}Configured Docker environment for production with real Proxmox data${NC}"
      fi
      
      docker compose up --build
      break
      ;;
    q|Q)
      echo -e "\n${BLUE}Exiting...${NC}"
      exit 0
      ;;
    *)
      echo -e "\n${YELLOW}Invalid option. Please try again.${NC}"
      ;;
  esac
done 