# <img src="src/public/logos/pulse-logo-256x256.png" alt="Pulse Logo" width="32" height="32" style="vertical-align: middle"> Pulse for Proxmox VE

[![GitHub release (latest by date)](https://img.shields.io/github/v/release/rcourtman/Pulse)](https://github.com/rcourtman/Pulse/releases/latest)
[![License](https://img.shields.io/github/license/rcourtman/Pulse)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/rcourtman/pulse)](https://hub.docker.com/r/rcourtman/pulse)

A lightweight monitoring application for Proxmox VE that displays real-time status for VMs and containers via a simple web interface.

![Pulse Dashboard](docs/images/01-dashboard.png)

### 📸 Screenshots

<details>
<summary><strong>Click to view more screenshots</strong></summary>

**Desktop Views:**
<div align="center">
<table>
<tr>
<td align="center"><strong>PBS Tab</strong></td>
<td align="center"><strong>Backups Tab</strong></td>
</tr>
<tr>
<td><img src="docs/images/02-pbs-view.png" alt="PBS View" width="400"/></td>
<td><img src="docs/images/03-backups-view.png" alt="Backups View" width="400"/></td>
</tr>
<tr>
<td align="center"><strong>Storage Tab</strong></td>
<td align="center"><strong>Line Graph Toggle</strong></td>
</tr>
<tr>
<td><img src="docs/images/04-storage-view.png" alt="Storage View" width="400"/></td>
<td><img src="docs/images/05-line-graph-toggle.png" alt="Line Graph Toggle View" width="400"/></td>
</tr>
</table>
</div>

**Mobile Views:**
<div align="center">
<table>
<tr>
<td align="center"><strong>Mobile Dashboard</strong></td>
<td align="center"><strong>Mobile PBS View</strong></td>
<td align="center"><strong>Mobile Backups View</strong></td>
</tr>
<tr>
<td><img src="docs/images/06-mobile-dashboard.png" alt="Mobile Dashboard" width="250"/></td>
<td><img src="docs/images/07-mobile-pbs-view.png" alt="Mobile PBS View" width="250"/></td>
<td><img src="docs/images/08-mobile-backups-view.png" alt="Mobile Backups View" width="250"/></td>
</tr>
</table>
</div>

</details>

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/rcourtman)

## 🚀 Quick Start

Choose your preferred installation method:

### 📦 **Easiest: Proxmox Community Scripts (Recommended)**
**One-command installation in a new LXC container:**
```bash
bash -c "$(wget -qLO - https://github.com/community-scripts/ProxmoxVE/raw/main/ct/pulse.sh)"
```
This will create a new LXC container and install Pulse automatically. Visit the [Community Scripts page](https://community-scripts.github.io/ProxmoxVE/scripts?id=pulse) for details.

### 🐳 **Docker Compose (Pre-built Image)**
**For existing Docker hosts:**
```bash
mkdir pulse-config && cd pulse-config
# Create docker-compose.yml (see Docker section)
docker compose up -d
# Configure via web interface at http://localhost:7655
```

### 🛠️ **Manual LXC Installation**
**For existing LXC containers:**
```bash
curl -sLO https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-pulse.sh
chmod +x install-pulse.sh
sudo ./install-pulse.sh
```

---

## 📋 Table of Contents
- [Quick Start](#-quick-start)
- [Prerequisites](#-prerequisites)
- [Configuration](#️-configuration)
  - [Environment Variables](#environment-variables)
  - [Alert System Configuration](#alert-system-configuration-optional)
  - [Custom Per-VM/LXC Alert Thresholds](#custom-per-vmlxc-alert-thresholds-optional)
  - [Webhook Notifications](#webhook-notifications-optional)
  - [Email Notifications](#email-notifications-optional)
  - [Creating a Proxmox API Token](#creating-a-proxmox-api-token)
  - [Creating a Proxmox Backup Server API Token](#creating-a-proxmox-backup-server-api-token)
  - [Required Permissions](#required-permissions)
- [Deployment Options](#-deployment-options)
  - [Proxmox Community Scripts](#proxmox-community-scripts-automated-lxc)
  - [Docker Compose](#docker-compose-recommended-for-existing-hosts)
  - [Manual LXC Installation](#manual-lxc-installation)
  - [Development Setup](#development-setup-docker-compose)
  - [Node.js (Development)](#️-running-the-application-nodejs-development)
- [Features](#-features)
- [System Requirements](#-system-requirements)
- [Updating Pulse](#-updating-pulse)
- [Privacy](#-privacy)
- [License](#-license)
- [Trademark Notice](#trademark-notice)
- [Support](#-support)
- [Troubleshooting](#-troubleshooting)
  - [Quick Fixes](#-quick-fixes)
  - [Diagnostic Tool](#diagnostic-tool)
  - [Common Issues](#common-issues)
  - [Notification Troubleshooting](#notification-troubleshooting)

## ✅ Prerequisites

Before installing Pulse, ensure you have:

**For Proxmox VE:**
- [ ] Proxmox VE 7.x or 8.x running
- [ ] Admin access to create API tokens
- [ ] Network connectivity between Pulse and Proxmox (ports 8006/8007)

**For Pulse Installation:**
- [ ] **Community Scripts**: Just a Proxmox host (handles everything automatically)
- [ ] **Docker**: Docker & Docker Compose installed
- [ ] **Manual LXC**: Existing Debian/Ubuntu LXC with internet access

---

## 🚀 Deployment Options

### Proxmox Community Scripts (Automated LXC)

**✨ Easiest method - fully automated LXC creation and setup:**

```bash
bash -c "$(wget -qLO - https://github.com/community-scripts/ProxmoxVE/raw/main/ct/pulse.sh)"
```

This script will:
- Create a new LXC container automatically
- Install all dependencies (Node.js, npm, etc.)
- Download and set up Pulse
- Set up systemd service

**After installation:** Access Pulse at `http://<lxc-ip>:7655` and configure via the web interface

Visit the [Community Scripts page](https://community-scripts.github.io/ProxmoxVE/scripts?id=pulse) for more details.

---

### Docker Compose (Recommended for Existing Hosts)

**For existing Docker hosts - uses pre-built image:**

**Prerequisites:**
- Docker ([Install Docker](https://docs.docker.com/engine/install/))
- Docker Compose ([Install Docker Compose](https://docs.docker.com/compose/install/))

**Steps:**

1.  **Create a Directory:** Make a directory for your Docker configuration files:
    ```bash
    mkdir pulse-config
    cd pulse-config
    ```
2.  **Create `docker-compose.yml` file:** Create a file named `docker-compose.yml` in this directory with the following content:
    ```yaml
    # docker-compose.yml
    services:
      pulse-server:
        image: rcourtman/pulse:latest # Pulls the latest pre-built image
        container_name: pulse
        restart: unless-stopped
        ports:
          # Map host port 7655 to container port 7655
          # Change the left side (e.g., "8081:7655") if 7655 is busy on your host
          - "7655:7655"
        volumes:
          # Persistent volume for configuration data
          # Configuration persists across container updates
          - pulse_config:/usr/src/app/config

    # Define persistent volumes
    volumes:
      pulse_config:
        driver: local
    ```
3.  **Run:** Start the container:
    ```bash
    docker compose up -d
    ```
4.  **Access and Configure:** Open your browser to `http://<your-docker-host-ip>:7655` and configure through the web interface.

---

### Manual LXC Installation

**For existing Debian/Ubuntu LXC containers:**

**Prerequisites:**
- A running Proxmox VE host
- An existing Debian or Ubuntu LXC container with network access to Proxmox
    - *Tip: Use [Community Scripts](https://community-scripts.github.io/ProxmoxVE/scripts?id=debian) to easily create one: `bash -c "$(curl -fsSL https://raw.githubusercontent.com/community-scripts/ProxmoxVE/main/ct/debian.sh)"`*

**Steps:**

1.  **Access LXC Console:** Log in to your LXC container (usually as `root`).
2.  **Download and Run Script:**
    ```bash
    # Ensure you are in a suitable directory, like /root or /tmp
    curl -sLO https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-pulse.sh
    chmod +x install-pulse.sh
    ./install-pulse.sh
    ```
3.  **Follow Prompts:** The script guides you through:
    *   Installing dependencies (`git`, `curl`, `nodejs`, `npm`, `sudo`).
    *   Setting up Pulse as a `systemd` service (`pulse-monitor.service`).
    *   Optionally enabling automatic updates via cron.
4.  **Access and Configure:** The script will display the URL (e.g., `http://<LXC-IP-ADDRESS>:7655`). Open this URL and configure via the web interface.

For update instructions, see the [Updating Pulse](#-updating-pulse) section.

---

### Development Setup (Docker Compose)

Use this method if you have cloned the repository and want to build and run the application from the local source code.

1.  **Get Files:** Clone the repository (`git clone https://github.com/rcourtman/Pulse.git && cd Pulse`)
2.  **Run:** `docker compose up --build -d` (The included `docker-compose.yml` uses the `build:` context by default).
3.  **Access and Configure:** Open your browser to `http://localhost:7655` (or your host IP if Docker runs remotely) and configure via the web interface.

## 🛠️ Configuration

Pulse features a comprehensive web-based configuration system accessible through the settings menu. No manual file editing required!

### Web Interface Configuration (Recommended)

**First-time Setup:**
- Access Pulse at `http://your-host:7655`
- The settings modal will automatically open for initial configuration
- Configure all your Proxmox VE and PBS servers through the intuitive web interface
- Test connections with built-in connectivity verification
- Save and reload configuration without restarting the application

**Ongoing Management:**
- Click the settings icon (⚙️) in the top-right corner anytime
- Add/modify multiple PVE and PBS endpoints
- Configure alert thresholds and service intervals
- All changes are applied immediately

### Environment Variables (Development/Advanced)

**Note:** Most users should use the web-based configuration interface. Environment variables are primarily for development and advanced deployment scenarios.

For development setups or infrastructure-as-code deployments, Pulse can also be configured using environment variables in a `.env` file.

#### Proxmox VE (Primary Environment)

These are the minimum required variables:
-   `PROXMOX_HOST`: URL of your Proxmox server (e.g., `https://192.168.1.10:8006`).
-   `PROXMOX_TOKEN_ID`: Your API Token ID (e.g., `user@pam!tokenid`).
-   `PROXMOX_TOKEN_SECRET`: Your API Token Secret.

Optional variables:
-   `PROXMOX_NODE_NAME`: A display name for this endpoint in the UI (defaults to `PROXMOX_HOST`).
-   `PROXMOX_ALLOW_SELF_SIGNED_CERTS`: Set to `true` if your Proxmox server uses self-signed SSL certificates. Defaults to `false`.
-   `PORT`: Port for the Pulse server to listen on. Defaults to `7655`.
-   `BACKUP_HISTORY_DAYS`: Number of days of backup history to display (defaults to `365` for full year calendar view).
-   *(Username/Password fallback exists but API Token is strongly recommended)*

#### Alert System Configuration (Optional)

Pulse includes a comprehensive alert system that monitors resource usage and system status:

```env
# Alert System Configuration
ALERT_CPU_ENABLED=true
ALERT_MEMORY_ENABLED=true
ALERT_DISK_ENABLED=true
ALERT_DOWN_ENABLED=true

# Alert thresholds (percentages)
ALERT_CPU_THRESHOLD=85
ALERT_MEMORY_THRESHOLD=90
ALERT_DISK_THRESHOLD=95

# Alert durations (milliseconds - how long condition must persist)
ALERT_CPU_DURATION=300000       # 5 minutes
ALERT_MEMORY_DURATION=300000    # 5 minutes  
ALERT_DISK_DURATION=600000      # 10 minutes
ALERT_DOWN_DURATION=60000       # 1 minute
```

Alert features include:
- Real-time notifications with toast messages
- Multi-severity alerts (Critical, Warning, Resolved)
- Duration-based triggering (alerts only fire after conditions persist)
- Automatic resolution when conditions normalize
- Alert history tracking
- Webhook and email notification support
- Alert acknowledgment and escalation

#### Custom Per-VM/LXC Alert Thresholds (Optional)

For advanced monitoring scenarios, Pulse supports custom alert thresholds on a per-VM/LXC basis through the web interface:

**Use Cases:**
- **Storage/NAS VMs**: Set higher memory thresholds (e.g., 95%/99%) for VMs that naturally use high memory for disk caching
- **Application Servers**: Set lower CPU thresholds (e.g., 70%/85%) for performance-critical applications  
- **Development VMs**: Set custom disk thresholds (e.g., 75%/90%) for early storage warnings

**Configuration:**
1. Navigate to **Settings → Custom Thresholds** tab
2. Click **"Add Custom Threshold"**
3. Select your VM/LXC from the dropdown
4. Configure custom CPU, Memory, and/or Disk thresholds
5. Save configuration

**Features:**
- **Migration-aware**: Thresholds follow VMs when they migrate between cluster nodes
- **Per-metric control**: Configure only the metrics you need (CPU, Memory, Disk)
- **Visual indicators**: VMs with custom thresholds show a blue "T" badge in the dashboard
- **Fallback behavior**: VMs without custom thresholds use global settings

***Note:** For a Proxmox cluster, you only need to provide connection details for **one** node. Pulse automatically discovers other cluster members.*

#### Webhook Notifications (Optional)

Pulse supports webhook notifications for alerts, compatible with Discord, Slack, Microsoft Teams, Home Assistant, and any service that accepts webhooks.

**Configuration via Web Interface:**
1. Navigate to **Settings → Notifications** tab
2. Enable "Webhook Notifications" toggle
3. Enter your webhook URL
4. Click "Test Webhook" to verify connectivity
5. Save configuration

**Built-in Setup Guide:**
- Click "How to get a webhook URL?" below the URL field for step-by-step instructions
- Pulse automatically detects and formats messages for Discord and Slack
- Other services receive standard JSON payloads

**Supported Services:**
- **Discord** - Rich embeds with color coding
- **Slack** - Formatted attachments
- **Microsoft Teams** - Adaptive cards
- **Home Assistant** - Trigger automations
- **Any webhook endpoint** - Standard JSON format

📖 **For detailed setup instructions, see [Webhook Setup Guide](docs/webhooks.md)**

**Features:**
- Rich embed formatting with color-coded severity levels
- Automatic retry on failure
- Dual payload format supporting multiple platforms
- Real-time alert notifications for:
  - Resource threshold violations (CPU, Memory, Disk)
  - VM/Container availability changes
  - Alert escalations
  - Alert resolutions

#### Email Notifications (Optional)

Configure SMTP email notifications for alerts:

**Configuration via Web Interface:**
1. Navigate to **Settings → Alerts** tab
2. Enable "Email Notifications"
3. Configure SMTP settings:
   - **SMTP Host**: Your email server (e.g., `smtp.gmail.com`)
   - **SMTP Port**: Usually 587 for TLS, 465 for SSL, 25 for unencrypted
   - **Username**: Your email address or username
   - **Password**: Your email password (use App Password for Gmail)
   - **From Address**: Sender email address
   - **To Addresses**: Recipient(s), comma-separated for multiple
   - **Use SSL**: Enable for SSL/TLS encryption
4. Click "Test Email" to verify configuration
5. Save settings

**Gmail Configuration Example:**
1. Enable 2-factor authentication on your Google account
2. Generate an App Password: Google Account → Security → App passwords
3. Use settings:
   - Host: `smtp.gmail.com`
   - Port: `587`
   - Username: Your Gmail address
   - Password: Your App Password (not regular password)
   - Use SSL: Enabled


#### Multiple Proxmox Environments (Optional)

To monitor separate Proxmox environments (e.g., different clusters, sites) in one Pulse instance, add numbered variables:

-   `PROXMOX_HOST_2`, `PROXMOX_TOKEN_ID_2`, `PROXMOX_TOKEN_SECRET_2`
-   `PROXMOX_HOST_3`, `PROXMOX_TOKEN_ID_3`, `PROXMOX_TOKEN_SECRET_3`
-   ...and so on.

Optional numbered variables also exist (e.g., `PROXMOX_ALLOW_SELF_SIGNED_CERTS_2`, `PROXMOX_NODE_NAME_2`).

#### Advanced Configuration Options

For performance tuning and specialized deployments:

```env
# Performance & Retention
BACKUP_HISTORY_DAYS=365          # Backup history retention (default: 365 days)

# Update System Configuration
UPDATE_CHANNEL_PREFERENCE=stable  # Force specific update channel (stable/rc)
UPDATE_TEST_MODE=false            # Enable test mode for update system

# Development Variables
NODE_ENV=development              # Enable development mode features
DEBUG=pulse:*                     # Enable debug logging for specific modules

# Docker Detection (automatically set)
DOCKER_DEPLOYMENT=true            # Automatically detected in Docker environments
```

**Performance Notes:**
- `BACKUP_HISTORY_DAYS` affects calendar heatmap visualization and memory usage
- Lower values improve performance for environments with extensive backup histories
- Debug logging should only be enabled for troubleshooting as it increases log verbosity

#### Proxmox Backup Server (PBS) (Optional)

To monitor PBS instances:

**Primary PBS Instance:**
-   `PBS_HOST`: URL of your PBS server (e.g., `https://192.168.1.11:8007`).
-   `PBS_TOKEN_ID`: Your PBS API Token ID (e.g., `user@pbs!tokenid`). See [Creating a Proxmox Backup Server API Token](#creating-a-proxmox-backup-server-api-token).
-   `PBS_TOKEN_SECRET`: Your PBS API Token Secret.
-   `PBS_NODE_NAME`: **Important!** The internal hostname of your PBS server (e.g., `pbs-server-01`). This is usually required for API token auth because the token might lack permission to auto-discover the node name. See details below.
-   `PBS_ALLOW_SELF_SIGNED_CERTS`: Set to `true` for self-signed certificates. Defaults to `false`.
-   `PBS_PORT`: PBS API port. Defaults to `8007`.

**Additional PBS Instances:**

To monitor multiple PBS instances, add numbered variables, starting with `_2`:

-   `PBS_HOST_2`, `PBS_TOKEN_ID_2`, `PBS_TOKEN_SECRET_2`
-   `PBS_HOST_3`, `PBS_TOKEN_ID_3`, `PBS_TOKEN_SECRET_3`
-   ...and so on.

Optional numbered variables also exist for additional PBS instances (e.g., `PBS_NODE_NAME_2`, `PBS_ALLOW_SELF_SIGNED_CERTS_2`, `PBS_PORT_2`). Each PBS instance, whether primary or additional, requires its respective `PBS_NODE_NAME` or `PBS_NODE_NAME_n` to be set if API token authentication is used and the token cannot automatically discover the node name.

<details>
<summary><strong>Why <code>PBS_NODE_NAME</code> (or <code>PBS_NODE_NAME_n</code>) is Required (Click to Expand)</strong></summary>

Pulse needs to query task lists specific to the PBS node (e.g., `/api2/json/nodes/{nodeName}/tasks`). It attempts to discover this node name automatically by querying `/api2/json/nodes`. However, this endpoint is often restricted for API tokens (returning a 403 Forbidden error), even for tokens with high privileges, unless the `Sys.Audit` permission is granted on the root path (`/`).

Therefore, **setting `PBS_NODE_NAME` in your `.env` file is the standard and recommended way** to ensure Pulse can correctly query task endpoints when using API token authentication. If it's not set and automatic discovery fails due to permissions, Pulse will be unable to fetch task data (backups, verifications, etc.).

**How to find your PBS Node Name:**
1.  **SSH:** Log into your PBS server via SSH and run `hostname`.
2.  **UI:** Log into the PBS web interface. The hostname is typically displayed on the Dashboard under Server Status.

Example: If your PBS connects via `https://minipc-pbs.lan:8007` but its internal hostname is `proxmox-backup-server`, set:
```env
PBS_HOST=https://minipc-pbs.lan:8007
PBS_NODE_NAME=proxmox-backup-server
```
</details>

### Creating a Proxmox API Token

Using an API token is the recommended authentication method.

<details>
<summary><strong>Steps to Create a PVE API Token (Click to Expand)</strong></summary>

1.  **Log in to the Proxmox VE web interface.**
2.  **Create a dedicated user** (optional but recommended):
    *   Go to `Datacenter` → `Permissions` → `Users`.
    *   Click `Add`. Enter a `User name` (e.g., "pulse-monitor"), set Realm to `Proxmox VE authentication server` (`pam`), set a password, ensure `Enabled`. Click `Add`.
3.  **Create an API token:**
    *   Go to `Datacenter` → `Permissions` → `API Tokens`.
    *   Click `Add`.
    *   Select the `User` (e.g., "pulse-monitor@pam") or `root@pam`.
    *   Enter a `Token ID` (e.g., "pulse").
    *   Leave `Privilege Separation` checked. Click `Add`.
    *   **Important:** Copy the `Secret` value immediately. It's shown only once.
4.  **Assign permissions (to User and Token):**
    *   Go to `Datacenter` → `Permissions`.
    *   **Add User Permission:** Click `Add` → `User Permission`. Path: `/`, User: `pulse-monitor@pam`, Role: `PVEAuditor`, check `Propagate`. Click `Add`.
    *   **Add Token Permission:** Click `Add` → `API Token Permission`. Path: `/`, API Token: `pulse-monitor@pam!pulse`, Role: `PVEAuditor`, check `Propagate`. Click `Add`.
    *   *Note: The `PVEAuditor` role at the root path (`/`) with `Propagate` is crucial.*
5.  **Update `.env`:** Set `PROXMOX_TOKEN_ID` (e.g., `pulse-monitor@pam!pulse`) and `PROXMOX_TOKEN_SECRET` (the secret you copied).

</details>

### Creating a Proxmox Backup Server API Token

If monitoring PBS, create a token within the PBS interface.

<details>
<summary><strong>Steps to Create a PBS API Token (Click to Expand)</strong></summary>

1.  **Log in to the Proxmox Backup Server web interface.**
2.  **Create a dedicated user** (optional but recommended):
    *   Go to `Configuration` → `Access Control` → `User Management`.
    *   Click `Add`. Enter `User ID` (e.g., "pulse-monitor@pbs"), set Realm (likely `pbs`), add password. Click `Add`.
3.  **Create an API token:**
    *   Go to `Configuration` → `Access Control` → `API Token`.
    *   Click `Add`.
    *   Select `User` (e.g., "pulse-monitor@pbs") or `root@pam`.
    *   Enter `Token Name` (e.g., "pulse").
    *   Leave `Privilege Separation` checked. Click `Add`.
    *   **Important:** Copy the `Secret` value immediately.
4.  **Assign permissions (to User and Token):**
    *   Go to `Configuration` → `Access Control` → `Permissions`.
    *   **Add User Permission:** Click `Add` → `User Permission`. Path: `/`, User: `pulse-monitor@pbs`, Role: `Audit`, check `Propagate`. Click `Add`.
    *   **Add API Token Permission:** Click `Add` → `API Token Permission`. Path: `/`, API Token: `pulse-monitor@pbs!pulse`, Role: `Audit`, check `Propagate`. Click `Add`.
    *   *Note: The `Audit` role at root path (`/`) with `Propagate` is crucial for both user and token.*
5.  **Update `.env`:** Set `PBS_TOKEN_ID` (e.g., `pulse-monitor@pbs!pulse`) and `PBS_TOKEN_SECRET`.

</details>

### Required Permissions

-   **Proxmox VE:** 
    - **Basic monitoring:** The `PVEAuditor` role assigned at path `/` with `Propagate` enabled.
    - **To view PVE backup files:** Additionally requires `PVEDatastoreAdmin` role on `/storage` (or specific storage paths).
    
    <details>
    <summary>Important: Storage Content Visibility (Click to Expand)</summary>
    
    Due to Proxmox API limitations, viewing backup files in storage requires elevated permissions:
    - `PVEAuditor` alone is NOT sufficient to list storage contents via API
    - You must grant `PVEDatastoreAdmin` role which includes `Datastore.Allocate` permission
    - This applies even for read-only access to backup listings
    
    To fix empty PVE backup listings:
    ```bash
    # Grant storage admin permissions to your API token
    pveum acl modify /storage --tokens user@realm!tokenname --roles PVEDatastoreAdmin
    ```
    </details>
    
    <details>
    <summary>Permissions included in PVEAuditor (Click to Expand)</summary>
    - `Datastore.Audit`
    - `Permissions.Read` (implicitly included)
    - `Pool.Audit`
    - `Sys.Audit`
    - `VM.Audit`
    </details>
    
-   **Proxmox Backup Server:** The `Audit` role assigned at path `/` with `Propagate` enabled is recommended.

### Running from Release Tarball

For users who prefer not to use Docker or the LXC script, pre-packaged release tarballs are available.

**Prerequisites:**
- Node.js (Version 18.x or later recommended)
- npm (comes with Node.js)
- `tar` command (standard on Linux/macOS, available via tools like 7-Zip or WSL on Windows)

**Steps:**

1.  **Download:** Go to the [Pulse GitHub Releases page](https://github.com/rcourtman/Pulse/releases/latest). Download the `pulse-vX.Y.Z.tar.gz` file for the desired release.
2.  **Extract:** Create a directory and extract the tarball:
    ```bash
    mkdir pulse-app
    cd pulse-app
    tar -xzf /path/to/downloaded/pulse-vX.Y.Z.tar.gz
    # This creates a directory like pulse-vX.Y.Z/
    cd pulse-vX.Y.Z
    ```
3.  **Run:** Start the application using npm:
    ```bash
    npm start
    ```
    *(Note: The tarball includes pre-installed production dependencies, so `npm install` is not typically required unless you encounter issues.)*
4.  **Access and Configure:** Open your browser to `http://<your-server-ip>:7655` and configure via the web interface.

### ️ Running the Application (Node.js - Development)

For development purposes or running directly from source, see the **[DEVELOPMENT.md](DEVELOPMENT.md)** guide. This involves cloning the repository, installing dependencies using `npm install` in both the root and `server` directories, and running `npm run dev` or `npm run start`.

## ✨ Features

### Core Monitoring
- Lightweight monitoring for Proxmox VE nodes, VMs, and Containers
- Real-time status updates via WebSockets
- Simple, responsive web interface with dark/light theme support
- Multi-environment PVE monitoring support (monitor multiple clusters/sites)
- Efficient polling: Stops API polling when no clients are connected

### Advanced Alert System
- **Configurable alert thresholds** for CPU, Memory, Disk, and VM/CT availability
- **Custom per-VM/LXC alert thresholds** (perfect for storage VMs, application servers, etc.)
- **Migration-aware thresholds** that follow VMs across cluster nodes
- **Multi-severity alerts**: Info, Warning, Critical, and Resolved states
- **Duration-based triggering** (alerts only fire after conditions persist)
- **Alert history tracking** with comprehensive metrics
- **Alert acknowledgment** and suppression capabilities
- **Alert escalation** for unacknowledged critical alerts

### Notification Systems
- **Webhook notifications** for Discord, Slack, and Microsoft Teams
  - Rich embed formatting with color-coded severity
  - Dual payload format support
  - Built-in webhook testing
- **Email notifications** via SMTP
  - Multiple recipient support
  - SSL/TLS encryption
  - Gmail App Password support
  - Test email functionality

### Enhanced Update System
- **Smart Version Switching** between stable and RC releases with clear commit differences
- **Consolidated Update Mechanism** using proven install script for reliability
- **Real-time Progress Tracking** with detailed commit information and GitHub links
- **Automatic Backup & Restore** of configuration during updates
- **Context-Aware Updates** showing exactly what changes with each version switch
- **Dual Update Channels** with persistent preference management

#### Update Channels
- **Stable Channel**: Production-ready releases (e.g., v3.27.1)
  - Thoroughly tested releases for production environments
  - Automatic updates only to stable versions
  - Recommended for critical infrastructure monitoring
- **RC Channel**: Release candidates with latest features (e.g., v3.28.0-rc1)
  - Early access to new features and improvements
  - Automated releases with each development commit
  - Perfect for testing and non-critical environments
- **Channel Persistence**: Your update preference is maintained across all updates
- **Smart Switching**: See exact commit differences when switching between channels

### Backup Monitoring
- **Comprehensive backup monitoring:**
  - Proxmox Backup Server (PBS) snapshots and tasks
  - PVE backup files on local and shared storage
  - VM/CT snapshot tracking with calendar heatmap visualization
- **Enhanced backup health card** with health score calculation
- **Recent coverage metrics** showing protection status
- **Backup type filtering** with styled badges

<details>
<summary><strong>Understanding Backup Types in Pulse</strong></summary>

Pulse monitors three distinct types of backups:

1. **PBS Backups** (Purple indicator ●)
   - Backups stored in Proxmox Backup Server
   - Accessed via PBS API with deduplication and verification features
   - Requires separate PBS API token configuration
   
2. **PVE Backups** (Orange indicator ●)
   - Backups stored on any Proxmox VE storage (NFS, CIFS, local, etc.)
   - All non-PBS backup storage is considered "PVE storage"
   - Accessed via Proxmox VE API
   
3. **Snapshots** (Yellow indicator ●)
   - VM/CT point-in-time snapshots (not full backups)
   - Stored locally on the Proxmox node

**Important:** If you have PBS configured as storage in Proxmox VE, those backups are accessed via the PBS API directly, not through PVE storage. This prevents double-counting of PBS backups.
</details>

### Performance & UI
- **Virtual scrolling** for handling large VM/container lists efficiently
- **Metrics history** with 1-hour retention using circular buffers
- **Network anomaly detection** with automatic baseline learning
- **Responsive design** optimized for desktop and mobile
- **UI scale adjustment** for different screen sizes
- **Persistent filter states** across sessions

### Management & Diagnostics
- **Built-in update manager** with web-based updates (non-Docker)
- **Comprehensive diagnostic tool** with API permission testing
- **Privacy-protected diagnostic exports** for troubleshooting
- **Real-time connectivity testing** for all configured endpoints
- **Automatic configuration validation**

### Deployment & Integration
- Docker support with pre-built images
- LXC installation script
- Proxmox Community Scripts integration
- systemd service management
- Automatic update capability via cron

## 🏗️ Architecture

### Technology Stack
- **Frontend**: Vue.js 3 with vanilla JavaScript modules
- **Backend**: Node.js 20+ with Express 5
- **Styling**: Tailwind CSS v3.4.4 with custom scrollbar plugin
- **Build System**: npm scripts with PostCSS and Tailwind compilation
- **Real-time Communication**: WebSocket integration with Socket.IO

### Project Structure
```
pulse/
├── src/public/          # Frontend application
│   ├── js/ui/          # Modular UI components (Vue.js)
│   ├── css/            # Styling and themes
│   └── output.css      # Compiled Tailwind styles
├── server/             # Backend API and services
│   ├── routes/         # Express route handlers
│   ├── services/       # Business logic modules
│   └── *.js           # Core server components
├── scripts/           # Installation and utility scripts
└── config/           # Configuration management
```

### Key Design Principles
- **Modular Architecture**: Clean separation between UI components and server modules
- **Performance Optimized**: Virtual scrolling, circular buffers, and efficient polling
- **Real-time Updates**: WebSocket-based live data streaming
- **Multi-platform Support**: Docker, LXC, and native deployment options
- **Configuration-driven**: Web-based configuration with automatic validation

## 💻 System Requirements

- **Node.js:** Version 18.x or later (if building/running from source).
- **NPM:** Compatible version with Node.js.
- **Docker & Docker Compose:** Latest stable versions (if using container deployment).
- **Proxmox VE:** Version 7.x or 8.x recommended.
- **Proxmox Backup Server:** Version 2.x or 3.x recommended (if monitored).
- **Web Browser:** Modern evergreen browser.

## 🔄 Updating Pulse

### Web-Based Updates (Non-Docker)

For non-Docker installations, Pulse includes a built-in update mechanism:

1. Open the Settings modal (gear icon in the top right)
2. Scroll to the "Software Updates" section
3. Click "Check for Updates"
4. If an update is available, review the release notes
5. Click "Apply Update" to install it automatically

The update process:
- Backs up your configuration files
- Downloads and applies the update
- Preserves your settings
- Automatically restarts the application

### Community Scripts LXC Installation

If you installed using the Community Scripts method, simply re-run the original installation command:

```bash
bash -c "$(wget -qLO - https://github.com/community-scripts/ProxmoxVE/raw/main/ct/pulse.sh)"
```

The script will detect the existing installation and update it automatically.

### Docker Compose Installation

Docker deployments must be updated by pulling the new image:

```bash
cd /path/to/your/pulse-config
docker compose pull
docker compose up -d
```

This pulls the latest image and recreates the container with the new version.

**Note:** The web-based update feature will detect Docker deployments and provide these instructions instead of attempting an in-place update.

### Manual LXC Installation

If you used the manual installation script, update by re-running it:

```bash
# Navigate to where you downloaded the script
cd /path/to/script/directory
./install-pulse.sh
```

Or run non-interactively (useful for automated updates):

```bash
./install-pulse.sh --update
```

**Managing the Service:**
- Check status: `sudo systemctl status pulse-monitor.service`
- View logs: `sudo journalctl -u pulse-monitor.service -f`
- Restart: `sudo systemctl restart pulse-monitor.service`

**Automatic Updates:**
If you enabled automatic updates during installation, they run via cron. Check logs in `/var/log/pulse_update.log`.

### Release Tarball Installation

To update a tarball installation:

1. Download the latest release from [GitHub Releases](https://github.com/rcourtman/Pulse/releases/latest)
2. Stop the current application
3. Extract the new tarball to a new directory
4. Start the application: `npm start`
5. Your configuration will be preserved automatically

### Development/Source Installation

If running from source code:

**For stable releases (production):**
```bash
cd /path/to/pulse
git checkout main
git pull origin main
npm install
npm run build:css
npm run start    # or your preferred restart method
```

**For development/RC versions:**
```bash
cd /path/to/pulse
git checkout develop
git pull origin develop
npm install
npm run build:css
npm run start    # or your preferred restart method
```

**Note:** 
- The development setup only requires npm install in the root directory, not in a separate server directory.
- The `develop` branch contains development work and release candidates.
- The `main` branch contains stable releases only.


## 🔒 Privacy

*   **No Data Collection:** Pulse does not collect or transmit any telemetry or user data externally.
*   **Local Communication:** Operates entirely between your environment and your Proxmox/PBS APIs.
*   **Credential Handling:** Credentials are used only for API authentication and are not logged or sent elsewhere.

## 🛡️ Security Best Practices

### API Token Security
- **Use dedicated service accounts** for API tokens instead of root accounts
- **Enable privilege separation** for all tokens to limit access scope
- **Regularly rotate API credentials** (quarterly or after personnel changes)
- **Audit token permissions** periodically to ensure least-privilege access
- **Monitor API access logs** for unusual activity patterns

### Network Security
- **Configure firewall rules** to restrict API access (ports 8006/8007) to necessary hosts only
- **Use SSL/TLS** for all API connections (avoid self-signed certificates in production)
- **Consider VPN access** for external monitoring setups
- **Implement network segmentation** to isolate monitoring traffic from production networks
- **Enable fail2ban** or similar tools on Proxmox hosts to prevent brute force attacks

### Deployment Security
- **Run Pulse with non-root user** when possible (LXC and manual installations)
- **Keep container/system updated** with latest security patches
- **Use configuration management** instead of hardcoded credentials
- **Secure webhook URLs** and email credentials with proper access controls
- **Monitor Pulse logs** for authentication failures or connection issues

### Proxmox Configuration
- **Disable unused APIs** and services on Proxmox hosts
- **Enable two-factor authentication** for Proxmox web interface access
- **Use strong passwords** for all Proxmox user accounts
- **Regularly update** Proxmox VE and PBS to latest stable versions
- **Configure proper backup encryption** for sensitive VM/CT data

## 📜 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file.

## ™️ Trademark Notice

Proxmox® and Proxmox VE® are registered trademarks of Proxmox Server Solutions GmbH. This project is not affiliated with or endorsed by Proxmox Server Solutions GmbH.

## ❤️ Support

File issues on the [GitHub repository](https://github.com/rcourtman/Pulse/issues).

If you find Pulse useful, consider supporting its development:
[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/rcourtman)

## ❓ Troubleshooting

### 🔧 Quick Fixes

**Can't access Pulse after installation?**
```bash
# Check if service is running
sudo systemctl status pulse-monitor.service

# Check what's listening on port 7655
sudo netstat -tlnp | grep 7655

# View recent logs
sudo journalctl -u pulse-monitor.service -f
```

**Empty dashboard or "No data" errors?**
1. **Check API Token:** Verify your `PROXMOX_TOKEN_ID` and `PROXMOX_TOKEN_SECRET` are correct
2. **Test connectivity:** Can you ping your Proxmox host from where Pulse is running?
3. **Check permissions:** Ensure token has `PVEAuditor` role on path `/` with `Propagate` enabled

**"Empty Backups Tab" with PBS configured?**
- Ensure `PBS Node Name` is configured in the settings modal
- Find hostname with: `ssh root@your-pbs-ip hostname`

**Docker container won't start?**
```bash
# Check container logs
docker logs pulse

# Restart container
docker compose down && docker compose up -d
```

### Diagnostic Tool

Pulse includes a comprehensive built-in diagnostic tool to help troubleshoot configuration and connectivity issues:

**Web Interface (Recommended):**
- The diagnostics icon appears automatically in the header when issues are detected
- Click the icon or navigate to `http://your-pulse-host:7655/diagnostics.html`
- The tool will automatically run diagnostics and provide:
  - **API Token Permission Testing** - Tests actual API permissions for VMs, containers, nodes, and datastores
  - **Configuration Validation** - Verifies all connection settings and required parameters
  - **Real-time Connectivity Tests** - Tests live connections to Proxmox VE and PBS instances
  - **Data Flow Analysis** - Shows discovered nodes, VMs, containers, and backup data
  - **Specific Actionable Recommendations** - Detailed guidance for fixing any issues found

**Key Features:**
- Tests use the same API endpoints as the main application for accuracy
- Provides exact permission requirements (e.g., `VM.Audit` on `/` for Proxmox)
- Shows counts of discovered resources (VMs, containers, nodes, backups)
- Identifies common misconfigurations like missing `PBS_NODE_NAME`
- **Privacy Protected**: Automatically sanitizes hostnames, IPs, and sensitive data before export
- Export diagnostic reports safe for sharing in GitHub issues or support requests

**Command Line:**
```bash
# If using the source code:
./scripts/diagnostics.sh

# The script will generate a detailed report and save it to a timestamped file
```

### Common Issues

*   **Proxmox Log File Growth / log2ram Issues:** 
    - **Update (v3.30.0+):** Pulse now uses the bulk `/cluster/resources` endpoint, reducing API calls by up to 95% while maintaining 2-second polling
    - **If you still experience log growth**, you can configure Proxmox logging: 
      
      **Option 1: Use tmpfs for pveproxy logs (Best for log2ram users)**
      ```bash
      # Add to /etc/fstab on your Proxmox host:
      tmpfs /var/log/pveproxy/ tmpfs defaults,uid=33,gid=33,size=1024m 0 0
      
      # Then mount it:
      mount /var/log/pveproxy/
      systemctl restart pveproxy
      ```
      
      **Option 2: Disable pveproxy access logging entirely**
      ```bash
      # On your Proxmox host, symlink to /dev/null:
      systemctl stop pveproxy
      rm -f /var/log/pveproxy/access.log
      ln -s /dev/null /var/log/pveproxy/access.log
      systemctl start pveproxy
      ```
      
      **Option 3: Aggressive logrotate configuration**
      ```bash
      # Edit /etc/logrotate.d/pve on your Proxmox host:
      /var/log/pveproxy/access.log {
          hourly          # Rotate every hour
          rotate 4        # Keep only 4 files
          maxsize 10M     # Force rotate at 10MB
          compress
          delaycompress
          notifempty
          missingok
          create 640 www-data www-data
      }
      
      # Force immediate rotation:
      logrotate -f /etc/logrotate.d/pve
      ```
      
      **Option 4: Exclude from log2ram**
      ```bash
      # Edit /etc/log2ram.conf and add to exclusion:
      LOG2RAM_PATH_EXCLUDE="/var/log/pveproxy"
      
      # Then restart log2ram:
      systemctl restart log2ram
      ```
    - **Note:** The pveproxy log path is hard-coded in Proxmox and cannot be changed. Pulse's 2-second polling provides real-time responsiveness - adjusting Proxmox logging is preferable to reducing polling frequency.
*   **Empty Backups Tab:** 
    - **PBS backups not showing:** Usually caused by missing `PBS Node Name` in the settings configuration. SSH to your PBS server and run `hostname` to find the correct value.
    - **PVE backups not showing:** Ensure your API token has `PVEDatastoreAdmin` role on `/storage` to view backup files. See the permissions section above.
*   **Pulse Application Logs:** Check container logs (`docker logs pulse_monitor`) or service logs (`sudo journalctl -u pulse-monitor.service -f`) for errors (401 Unauthorized, 403 Forbidden, connection refused, timeout).
*   **Configuration Issues:** Use the settings modal to verify all connection details. Test connections with the built-in connectivity tester before saving. Ensure no placeholder values remain.
*   **Network Connectivity:** Can the machine running Pulse reach the PVE/PBS hostnames/IPs and ports (usually 8006 for PVE, 8007 for PBS)? Check firewalls.
*   **API Token Permissions:** Ensure the correct roles (`PVEAuditor` for PVE, `Audit` for PBS) are assigned at the root path (`/`) with `Propagate` enabled in the respective UIs.

### Notification Troubleshooting

**Webhook notifications not working?**
- **Test the webhook:** Use the "Test Webhook" button in settings to verify connectivity
- **Check the URL format:** Ensure you're using the full webhook URL including protocol (https://)
- **Firewall rules:** Verify Pulse can reach Discord/Slack/Teams servers (outbound HTTPS)
- **Check logs:** Look for webhook errors in application logs

**Email notifications not sending?**
- **Test configuration:** Use the "Test Email" button to verify SMTP settings
- **Gmail issues:** 
  - Must use App Password, not regular password
  - Enable "Less secure app access" or use App Passwords with 2FA
- **Port issues:** Try different ports (587 for TLS, 465 for SSL, 25 for unencrypted)
- **Firewall:** Ensure outbound SMTP traffic is allowed
- **Authentication:** Double-check username/password, some servers require full email address
