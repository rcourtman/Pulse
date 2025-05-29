#!/bin/bash
# Pulse CSS Fix Script
# This script fixes CSS issues in broken Pulse installations
# where the frontend shows no styling due to CSS MIME type errors

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Default installation path
PULSE_DIR="/opt/pulse-proxmox"

print_info "Pulse CSS Fix Script"
print_info "This script fixes CSS issues where the frontend has no styling"
echo ""

# Check if Pulse directory exists
if [ ! -d "$PULSE_DIR" ]; then
    print_error "Pulse installation directory not found at $PULSE_DIR"
    print_info "Please specify the correct Pulse installation path:"
    read -p "Enter Pulse installation path: " PULSE_DIR
    
    if [ ! -d "$PULSE_DIR" ]; then
        print_error "Directory $PULSE_DIR does not exist. Exiting."
        exit 1
    fi
fi

print_info "Using Pulse installation at: $PULSE_DIR"

# Check if this is a Pulse installation
if [ ! -f "$PULSE_DIR/package.json" ] || [ ! -f "$PULSE_DIR/server/index.js" ]; then
    print_error "This doesn't appear to be a valid Pulse installation"
    print_error "Missing package.json or server/index.js"
    exit 1
fi

# Change to Pulse directory
cd "$PULSE_DIR" || {
    print_error "Failed to change to $PULSE_DIR"
    exit 1
}

print_info "Checking current CSS status..."

# Check if output.css exists and has content
if [ ! -f "src/public/output.css" ]; then
    print_warning "output.css file is missing"
    CSS_MISSING=1
elif [ ! -s "src/public/output.css" ]; then
    print_warning "output.css file is empty"
    CSS_EMPTY=1
else
    # Check if CSS contains actual CSS content (not HTML error page)
    if head -1 "src/public/output.css" | grep -q "<!DOCTYPE\|<html\|<head"; then
        print_warning "output.css contains HTML instead of CSS (corrupted)"
        CSS_CORRUPTED=1
    else
        print_info "output.css appears to contain valid CSS"
        CSS_VALID=1
    fi
fi

# If CSS is valid, check if there's actually a problem
if [ "$CSS_VALID" = "1" ]; then
    print_info "CSS file appears to be valid. You may not need this fix."
    print_info "Are you experiencing frontend styling issues? (y/N)"
    read -r response
    if [[ ! "$response" =~ ^[Yy]$ ]]; then
        print_info "Exiting without changes."
        exit 0
    fi
fi

print_info "Attempting to fix CSS..."

# Method 1: Try to rebuild CSS if we have the tools
if [ -f "src/tailwind.config.js" ] && [ -f "src/index.css" ]; then
    print_info "Found Tailwind config and source CSS, attempting rebuild..."
    
    # Check if we have npm and tailwindcss
    if command -v npm >/dev/null 2>&1; then
        # Try to rebuild
        if npm run build:css >/dev/null 2>&1; then
            print_success "CSS rebuilt successfully using npm run build:css"
            
            # Verify the rebuild worked
            if [ -f "src/public/output.css" ] && [ -s "src/public/output.css" ]; then
                if ! head -1 "src/public/output.css" | grep -q "<!DOCTYPE\|<html\|<head"; then
                    print_success "CSS fix completed successfully!"
                    print_info "Please refresh your browser (Ctrl+F5 or Cmd+Shift+R) to see the changes"
                    exit 0
                fi
            fi
        else
            print_warning "npm run build:css failed (likely missing dev dependencies)"
        fi
    else
        print_warning "npm command not found"
    fi
fi

# Method 2: Download pre-built CSS from latest release
print_info "Attempting to download pre-built CSS from latest release..."

# Create backup of current CSS
if [ -f "src/public/output.css" ]; then
    cp "src/public/output.css" "src/public/output.css.backup.$(date +%s)"
    print_info "Current CSS backed up"
fi

# Try to download CSS from GitHub
if command -v curl >/dev/null 2>&1; then
    print_info "Downloading CSS from latest Pulse release..."
    
    # Get latest release info
    LATEST_RELEASE=$(curl -s https://api.github.com/repos/rcourtman/Pulse/releases/latest | grep -o '"tag_name": "[^"]*' | cut -d'"' -f4)
    
    if [ -n "$LATEST_RELEASE" ]; then
        print_info "Latest release: $LATEST_RELEASE"
        
        # Download and extract just the CSS file
        TEMP_DIR=$(mktemp -d)
        cd "$TEMP_DIR"
        
        # Download tarball
        if curl -sL "https://github.com/rcourtman/Pulse/releases/download/$LATEST_RELEASE/pulse-${LATEST_RELEASE#v}.tar.gz" -o pulse.tar.gz; then
            # Extract just the CSS file
            if tar -xzf pulse.tar.gz --wildcards "*/src/public/output.css" 2>/dev/null; then
                # Find and copy the CSS file
                CSS_FILE=$(find . -name "output.css" -path "*/src/public/*" | head -1)
                if [ -n "$CSS_FILE" ] && [ -f "$CSS_FILE" ]; then
                    cp "$CSS_FILE" "$PULSE_DIR/src/public/output.css"
                    print_success "CSS downloaded and installed from release $LATEST_RELEASE"
                    
                    # Clean up temp directory
                    cd "$PULSE_DIR"
                    rm -rf "$TEMP_DIR"
                    
                    print_success "CSS fix completed successfully!"
                    print_info "Please refresh your browser (Ctrl+F5 or Cmd+Shift+R) to see the changes"
                    exit 0
                else
                    print_error "Could not find CSS file in downloaded release"
                fi
            else
                print_error "Failed to extract CSS from downloaded release"
            fi
        else
            print_error "Failed to download release tarball"
        fi
        
        # Clean up temp directory
        cd "$PULSE_DIR"
        rm -rf "$TEMP_DIR"
    else
        print_error "Could not determine latest release version"
    fi
else
    print_warning "curl command not found, cannot download CSS"
fi

# Method 3: Create minimal CSS as last resort
print_warning "All automated fixes failed. Creating minimal CSS as last resort..."

# Create a basic CSS file that will at least make the page usable
cat > "src/public/output.css" << 'EOF'
/* Minimal CSS for Pulse - Emergency Fix */
* { box-sizing: border-box; }
body { font-family: system-ui, sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }
.container { max-width: 1200px; margin: 0 auto; }
.card { background: white; border-radius: 8px; padding: 20px; margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
.btn { padding: 8px 16px; border: none; border-radius: 4px; cursor: pointer; }
.btn-primary { background: #3b82f6; color: white; }
.text-red-600 { color: #dc2626; }
.text-green-600 { color: #16a34a; }
.text-yellow-600 { color: #ca8a04; }
.hidden { display: none; }
.flex { display: flex; }
.grid { display: grid; }
.gap-4 { gap: 1rem; }
EOF

print_warning "Created minimal emergency CSS"
print_warning "This provides basic styling but is not the full Pulse theme"
print_info "Consider running the Pulse installer again or manually rebuilding CSS"

print_info "Please refresh your browser (Ctrl+F5 or Cmd+Shift+R) to see the changes"
print_info "CSS fix script completed"