# <img src="src/public/logos/pulse-logo-256x256.png" alt="Pulse Logo" width="32" height="32" style="vertical-align: middle"> Pulse for Proxmox VE

[![GitHub release (latest by date)](https://img.shields.io/github/v/release/rcourtman/Pulse)](https://github.com/rcourtman/Pulse/releases/latest)
[![License](https://img.shields.io/github/license/rcourtman/Pulse)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/rcourtman/pulse)](https://hub.docker.com/r/rcourtman/pulse)

A lightweight monitoring application for Proxmox VE that displays real-time status for VMs and containers via a simple web interface.

![Pulse Dashboard](docs/images/01-dashboard.png)

### 📸 Screenshots

<details>
<summary><strong>Click to view more screenshots</strong></summary>

<div align="center">
<table>
<tr>
<td align="center"><strong>Real-time Charts</strong></td>
<td align="center"><strong>Alert Thresholds</strong></td>
</tr>
<tr>
<td><img src="docs/images/05-charts-view.png" alt="Real-time performance charts" width="400"/></td>
<td><img src="docs/images/06-alerts-view.png" alt="Alert threshold configuration" width="400"/></td>
</tr>
<tr>
<td align="center"><strong>Storage Overview</strong></td>
<td align="center"><strong>Backup Management</strong></td>
</tr>
<tr>
<td><img src="docs/images/03-storage-view.png" alt="Storage overview" width="400"/></td>
<td><img src="docs/images/04-backups-view.png" alt="Backup management" width="400"/></td>
</tr>
<tr>
<td align="center" colspan="2"><strong>Proxmox Backup Server Integration</strong></td>
</tr>
<tr>
<td colspan="2" align="center"><img src="docs/images/02-pbs-view.png" alt="Unified backup view with PBS integration" width="600"/></td>
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
- [Contributing](#-contributing)
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
- [ ] Admin access to create API tokens (root not required - see [Security Best Practices](#security-best-practices-non-root-setup))
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
          # Persistent volume for metrics data, alert rules, and thresholds
          - pulse_data:/usr/src/app/data

    # Define persistent volumes
    volumes:
      pulse_config:
        driver: local
      pulse_data:
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

### 🔒 PBS Push Mode (For Isolated Servers)

If you have PBS servers behind firewalls or in isolated networks that can't be reached by Pulse:

1. **Enable on Pulse:** Add `PULSE_PUSH_API_KEY=your-secure-key` to your Pulse environment
2. **Install Agent on PBS:** Run on each isolated PBS server:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-pulse-agent.sh | bash
   ```
3. **Configure & Start:** Edit `/etc/pulse-agent/pulse-agent.env` with your settings and start the agent

See [PBS Push Mode Documentation](docs/PBS_PUSH_MODE.md) for detailed setup instructions.

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

**Features:**
- Rich embed formatting with color-coded severity levels
- Smart alert batching to prevent webhook spam
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

# Embedding Support
ALLOW_EMBEDDING=false            # Allow Pulse to be embedded in iframes (default: false)

# Update System Configuration
UPDATE_CHANNEL_PREFERENCE=stable  # Force specific update channel (stable/rc)
UPDATE_TEST_MODE=false            # Enable test mode for update system

# Development Variables
NODE_ENV=development              # Enable development mode features
DEBUG=pulse:*                     # Enable debug logging for specific modules

# Docker Detection (automatically set)
DOCKER_DEPLOYMENT=true            # Automatically detected in Docker environments
```

#### Iframe Embedding Support

Pulse can be embedded in other web applications (like Homepage, Organizr, or custom dashboards) using iframes.

**To enable embedding:**
1. Set `ALLOW_EMBEDDING=true` in your `.env` file
2. Restart Pulse
3. Embed using standard HTML iframe:
   ```html
   <iframe src="http://your-pulse-host:7655" width="100%" height="600"></iframe>
   ```

**Security Considerations:**
- By default, embedding is disabled to prevent clickjacking attacks
- When enabled, Pulse uses `X-Frame-Options: SAMEORIGIN` to allow embedding only from the same origin
- The Content Security Policy is also adjusted to permit iframe embedding
- Only enable this feature if you trust the applications that will embed Pulse

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
-   `PBS_ALLOW_SELF_SIGNED_CERTS`: Set to `true` for self-signed certificates. Defaults to `false`.
-   `PBS_PORT`: PBS API port. Defaults to `8007`.

**Additional PBS Instances:**

To monitor multiple PBS instances, add numbered variables, starting with `_2`:

-   `PBS_HOST_2`, `PBS_TOKEN_ID_2`, `PBS_TOKEN_SECRET_2`
-   `PBS_HOST_3`, `PBS_TOKEN_ID_3`, `PBS_TOKEN_SECRET_3`
-   ...and so on.

Optional numbered variables also exist for additional PBS instances (e.g., `PBS_ALLOW_SELF_SIGNED_CERTS_2`, `PBS_PORT_2`).

### Creating a Proxmox API Token

> ⚠️ **IMPORTANT:** Token permissions are the #1 cause of blank dashboards. With privilege separation disabled, the token inherits permissions from the user. With privilege separation enabled (default), you must set permissions on the token itself. Most users should DISABLE privilege separation for simplicity!

Using an API token is the recommended authentication method. **Never use root tokens in production** - see [Security Best Practices](#security-best-practices-non-root-setup) below.

<details>
<summary><strong>Steps to Create a PVE API Token (Click to Expand)</strong></summary>

1.  **Log in to the Proxmox VE web interface.**
2.  **Create a dedicated user** (optional but recommended):
    *   Go to `Datacenter` → `Permissions` → `Users`.
    *   Click `Add`. Enter a `User name` (e.g., "pulse-monitor"), set Realm to `Linux PAM standard authentication` (`pam`), ensure `Enabled` is checked. Click `Add`.
    *   Note: PAM users don't need a password for API token usage. The token will authenticate independently.
3.  **Create an API token:**
    *   Go to `Datacenter` → `Permissions` → `API Tokens`.
    *   Click `Add`.
    *   Select the `User` (e.g., "pulse-monitor@pam") or `root@pam`.
    *   Enter a `Token ID` (e.g., "pulse").
    *   **Uncheck `Privilege Separation`** (recommended for simplicity - token inherits user permissions). Click `Add`.
    *   **Important:** Copy the `Secret` value immediately. It's shown only once.
    *   **Note:** Creating a token does NOT automatically grant it permissions. You MUST complete step 4 below.
4.  **Assign permissions (CRITICAL STEP - Often Missed):**
    *   Go to `Datacenter` → `Permissions`.
    *   **Recommended (with Privilege Separation DISABLED):** Click `Add` → `User Permission`.
        - Path: `/`
        - User: `pulse-monitor@pam` (the USER, not the token!)
        - Role: `PVEAuditor`
        - Propagate: Must be checked
        - Click `Add`
    *   **Why disable privilege separation?** With it disabled, the token inherits all permissions from the user. This is simpler - you only need to set permissions once on the user.
    *   **If you keep privilege separation enabled:** You must set permissions on BOTH the user AND the token (using "API Token Permission" instead).
5.  **Update `.env`:** Set `PROXMOX_TOKEN_ID` (e.g., `pulse-monitor@pam!pulse`) and `PROXMOX_TOKEN_SECRET` (the secret you copied).

</details>

### Creating a Proxmox Backup Server API Token

If monitoring PBS, create a token within the PBS interface.

<details>
<summary><strong>Steps to Create a PBS API Token (Click to Expand)</strong></summary>

**🔒 Security Best Practice:** Pulse only needs `DatastoreAudit` permission (read-only access). Never grant `Admin` role for monitoring purposes.

> **Quick Setup (Recommended):**
> ```bash
> # Create user and token
> proxmox-backup-manager user create pulse-monitor@pbs --password 'ChangeMe123'
> proxmox-backup-manager user generate-token pulse-monitor@pbs monitoring
> # Grant read-only access
> proxmox-backup-manager acl update /datastore DatastoreAudit --auth-id 'pulse-monitor@pbs!monitoring'
> ```

1.  **Log in to the Proxmox Backup Server web interface.**
2.  **Create a dedicated user** (strongly recommended for security):
    *   Go to `Configuration` → `Access Control` → `User Management`.
    *   Click `Add`. Enter `User ID` (e.g., "pulse-monitor"), set Realm to `Proxmox Backup authentication server` (`pbs`). Click `Add`.
    *   Note: For PBS realm users, you'll need to set a password. For PAM users, no password is needed for API token usage.
    *   **⚠️ Avoid using `admin@pbs`** - This is the built-in administrative account with full privileges. Create a dedicated monitoring user instead.
3.  **Create an API token:**
    *   Go to `Configuration` → `Access Control` → `API Token`.
    *   Click `Add`.
    *   Select `User` (e.g., "pulse-monitor@pbs"). Avoid using `root@pam` or `admin@pbs` for production.
    *   Enter `Token Name` (e.g., "pulse").
    *   **Uncheck `Privilege Separation`** (recommended for simplicity - token inherits user permissions). Click `Add`.
    *   **Important:** Copy the `Secret` value immediately.
4.  **Assign permissions (CRITICAL STEP):**
    *   **With privilege separation disabled (recommended):** Set permissions on the USER only
    *   **With privilege separation enabled:** Set permissions on BOTH the USER and TOKEN
    *   **⚠️ Security Note:** Pulse only needs read-only access. Avoid granting Admin role unless absolutely necessary.
    
    *   **Recommended: Minimal read-only permissions**
    ```bash
    # Grant DatastoreAudit for read-only monitoring (THIS IS ALL PULSE NEEDS!)
    proxmox-backup-manager acl update /datastore DatastoreAudit --auth-id 'pulse-monitor@pbs!pulse'
    
    # Or for specific datastores only:
    proxmox-backup-manager acl update /datastore/main DatastoreAudit --auth-id 'pulse-monitor@pbs!pulse'
    ```
    
    *   **Not Recommended: Full admin access (only if troubleshooting)**
    ```bash
    # ⚠️ WARNING: This grants full read/write/delete permissions!
    # Only use temporarily for troubleshooting, then switch to DatastoreAudit
    proxmox-backup-manager acl update / Admin --auth-id 'user@realm!token'
    ```
5.  **Update `.env`:** Set `PBS_TOKEN_ID` (e.g., `pulse-monitor@pbs!pulse`) and `PBS_TOKEN_SECRET`.

</details>

### Required Permissions

-   **Proxmox VE:** 
    - **Basic monitoring:** The `PVEAuditor` role assigned at path `/` with `Propagate` enabled.
    - **To view PVE backup files:** Additionally requires `PVEDatastoreAdmin` role on `/storage` (or specific storage paths).
    
    <details>
    <summary>Important: Storage Content Visibility (Click to Expand)</summary>
    
    Due to Proxmox API limitations, viewing backup files in storage requires:
    - `PVEAuditor` alone is NOT sufficient to list storage contents via API
    - You need the `Datastore.Allocate` permission (included in `PVEDatastoreAdmin` role)
    - This applies even for read-only access to backup listings
    
    **Critical: Token Privilege Separation**
    
    How you grant permissions depends on your token's privilege separation setting:
    
    **Option 1: Token without privilege separation (recommended, simpler)**
    ```bash
    # Create token with --privsep 0
    # Set permissions on the USER only
    pveum acl modify / --users user@realm --roles PVEAuditor
    pveum acl modify /storage --users user@realm --roles PVEDatastoreAdmin
    ```
    
    **Option 2: Token with privilege separation (more complex)**
    ```bash
    # If you created token with --privsep 1 (default)
    # Must set permissions on BOTH user AND token
    pveum acl modify / --users user@realm --roles PVEAuditor
    pveum acl modify /storage --users user@realm --roles PVEDatastoreAdmin
    pveum acl modify / --tokens user@realm!tokenname --roles PVEAuditor
    pveum acl modify /storage --tokens user@realm!tokenname --roles PVEDatastoreAdmin
    ```
    
    **Why this matters:** Tokens with `--privsep 0` inherit all permissions from the user automatically. Tokens with `--privsep 1` require explicit permissions set on both the user AND the token - the token can only have permissions that the user also has.
    </details>
    
    <details>
    <summary>Permissions included in PVEAuditor (Click to Expand)</summary>
    - `Datastore.Audit`
    - `Mapping.Audit`
    - `Pool.Audit`
    - `SDN.Audit`
    - `Sys.Audit`
    - `VM.Audit`
    </details>
    
-   **Proxmox Backup Server:** Basic read permissions are sufficient. Either:
    - `Datastore.Audit` on specific datastores you want to monitor
    - `Datastore.Audit` at path `/` with `Propagate` for all datastores
    - Note: Pulse automatically discovers PBS namespaces and configuration
    - **IMPORTANT for PBS:** Unlike PVE, PBS tokens do NOT inherit permissions from users. You must explicitly grant permissions to the token using `--auth-id 'user@realm!token'`
    - **PBS Namespaces:** Pulse automatically discovers and displays backups from all namespaces (e.g., if your backups are organized in namespaces like `production`, `development`, `node1`, etc., they will all be shown)

### Security Best Practices (Non-Root Setup)

<details>
<summary><strong>🔒 How to Avoid Using Root Tokens (Click to Expand)</strong></summary>

Using root tokens is convenient but violates the principle of least privilege. Here's how to set up Pulse securely without root access:

#### Recommended: Dedicated User WITHOUT Privilege Separation

This is the simplest approach:

```bash
# 1. Create a dedicated monitoring user
pveum user add pulse-monitor@pam --comment "Pulse monitoring user"

# 2. Create token WITHOUT privilege separation (simpler)
pveum user token add pulse-monitor@pam monitoring --privsep 0

# 3. Set permissions on the USER only (token inherits them)
pveum acl modify / --users pulse-monitor@pam --roles PVEAuditor
pveum acl modify /storage --users pulse-monitor@pam --roles PVEDatastoreAdmin

# 4. Use in Pulse .env:
# PROXMOX_TOKEN_ID=pulse-monitor@pam!monitoring
# PROXMOX_TOKEN_SECRET=<your-token-secret>
```

#### Alternative: Token With Privilege Separation

More complex but provides finer control:

```bash
# 1. Create user
pveum user add pulse-monitor@pam --comment "Pulse monitoring user"

# 2. Create token WITH privilege separation
pveum user token add pulse-monitor@pam monitoring --privsep 1

# 3. Set permissions on BOTH USER AND TOKEN
pveum acl modify / --users pulse-monitor@pam --roles PVEAuditor
pveum acl modify /storage --users pulse-monitor@pam --roles PVEDatastoreAdmin
pveum acl modify / --tokens pulse-monitor@pam!monitoring --roles PVEAuditor
pveum acl modify /storage --tokens pulse-monitor@pam!monitoring --roles PVEDatastoreAdmin
```

#### Minimal Permissions Setup

For environments requiring strict access control:

```bash
# Create user with minimal permissions
pveum user add pulse-readonly@pve --comment "Pulse read-only"
pveum user token add pulse-readonly@pve monitor --privsep 1

# Grant basic monitoring (no backup visibility)
pveum acl modify / --users pulse-readonly@pve --roles PVEAuditor

# For backup visibility on specific storages only:
pveum acl modify /storage/local --users pulse-readonly@pve --roles PVEDatastoreAdmin
pveum acl modify /storage/nas --users pulse-readonly@pve --roles PVEDatastoreAdmin
# (Repeat for each storage you want to monitor)
```

#### PBS Non-Root Setup

PBS requires creating a dedicated monitoring user with minimal permissions:

```bash
# 1. Create a dedicated monitoring user (NOT admin@pbs!)
proxmox-backup-manager user create pulse-monitor@pbs --password 'SecurePassword123'

# 2. Create API token for the monitoring user
proxmox-backup-manager user generate-token pulse-monitor@pbs monitoring

# 3. Grant ONLY read permissions to the TOKEN
# This is all Pulse needs - no Admin role required!
proxmox-backup-manager acl update /datastore DatastoreAudit --auth-id 'pulse-monitor@pbs!monitoring'

# 4. Optional: For specific datastores only (even more restrictive):
proxmox-backup-manager acl update /datastore/main DatastoreAudit --auth-id 'pulse-monitor@pbs!monitoring'
proxmox-backup-manager acl update /datastore/backup DatastoreAudit --auth-id 'pulse-monitor@pbs!monitoring'
```

**⚠️ Important Security Notes:**
- Never use `admin@pbs` for monitoring - it has full administrative privileges
- `DatastoreAudit` provides all the read access Pulse needs
- Avoid granting `Admin` role unless troubleshooting
- The password for the user is only needed during creation, not for API token usage

#### Migrating from Root to Non-Root

If you're currently using root and want to switch:

```bash
# 1. Check current permissions in web diagnostics
# http://your-pulse-ip:7655/diagnostics.html

# 2. Create new non-root setup (see above)

# 3. Verify the new token works in diagnostics

# 4. Update .env and restart Pulse

# 5. After confirming it works, remove the old root token:
pveum user token remove root@pam your-token
```

#### Common Security Mistakes

1. **Using root tokens** - Gives unnecessary full access
2. **Not understanding privilege separation** - With it disabled, set permissions on user only; with it enabled, set permissions on BOTH user and token
3. **Only setting permissions on user with privsep=1** - Must also set on token!
4. **Granting Admin roles** - PVEAuditor + PVEDatastoreAdmin is sufficient

#### Quick Decision Guide

- **Simplest setup?** → Token with privsep=0 (disabled)
- **Maximum control?** → Token with privsep=1 (enabled) 
- **Minimal permissions?** → Restrict to specific storages
- **Multiple Pulse instances?** → Create separate tokens for each

</details>

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

For development purposes or running directly from source, see the **[DEVELOPMENT.md](DEVELOPMENT.md)** guide. This involves cloning the repository, installing dependencies using `npm install`, and running `npm run dev` or `npm run start`.

## ✨ Features

### Core Monitoring
- Lightweight monitoring for Proxmox VE nodes, VMs, and Containers
- Real-time status updates via WebSockets
- Simple, responsive web interface with dark/light theme support
- Multi-environment PVE monitoring support (monitor multiple clusters/sites)
- Efficient polling: Stops API polling when no clients are connected
- **PBS Push Mode**: Monitor isolated/firewalled PBS servers via agent (NEW)
  - Perfect for air-gapped or DMZ backup servers
  - Agent pushes metrics over HTTPS (no incoming connections needed)
  - Visual indicators for push vs pull connections
- **Streamlined tab structure:**
  - **Main**: Dashboard with VM/CT status overview
  - **Storage**: Storage usage and health monitoring
  - **Backups**: Unified view of all backups, snapshots, and PBS data

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
- **Stable Channel**: Production-ready releases
  - Thoroughly tested releases for production environments
  - Automatic updates only to stable versions
  - Recommended for critical infrastructure monitoring
- **RC Channel**: Release candidates with latest features
  - Early access to new features and improvements
  - Automated releases with each development commit
  - Perfect for testing and non-critical environments
- **Channel Persistence**: Your update preference is maintained across all updates
- **Smart Switching**: See exact commit differences when switching between channels

### Backup Monitoring
- **Comprehensive backup monitoring with unified view:**
  - **All backup types in one place**: VM/CT snapshots, PBS backups, and PVE backups
  - **Smart filtering**: Filter by backup type (snapshot, local, remote) or view all
  - **Real-time search and sorting** across all backup data
- **Real-time search and filtering** across all backup data
- **Column sorting** for easy data organization
- **Unified API** for streamlined PBS and PVE backup data access
- **WebSocket updates** for live data without UI flashing

<details>
<summary><strong>Understanding Backup Types in Pulse</strong></summary>

Pulse monitors three distinct types of backups, all unified in a single comprehensive view:

1. **PBS Backups** (Purple indicator ● - Remote backups)
   - Full backups stored in Proxmox Backup Server
   - Accessed via PBS API with deduplication and verification features
   - Requires separate PBS API token configuration
   - Includes PBS server status monitoring
   
2. **PVE Backups** (Orange indicator ● - Local backups)
   - Full backups stored on any Proxmox VE storage (NFS, CIFS, local, etc.)
   - All non-PBS backup storage is considered "PVE storage"
   - Accessed via Proxmox VE API
   
3. **Snapshots** (Yellow indicator ● - Point-in-time snapshots)
   - VM/CT point-in-time snapshots (not full backups)
   - Stored locally on the Proxmox node
   - Quick restore points for development and testing

**Unified Backup View:**
- **Filter by type**: View all backups or filter by Snapshot, Local (PVE), or Remote (PBS)
- **Smart search**: Search across all backup types simultaneously
- **Consistent interface**: Same powerful sorting and filtering for all backup types

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

**Note:** If the in-app updater fails or crashes, use the manual update method below.

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

### Manual Installation Updates

If you installed Pulse manually (not using Docker or Community Scripts):

#### Using the Update Flag (Recommended)
```bash
sudo /opt/pulse/scripts/install-pulse.sh --update
```

This will automatically download and install the latest version while preserving your configuration.

#### Alternative Method
If the above doesn't work, you can re-download and run the installer:

```bash
# Download the latest installer
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-pulse.sh > install-pulse.sh
chmod +x install-pulse.sh
sudo ./install-pulse.sh
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
- The `develop` branch shows dynamic RC versions (e.g., "3.24.0-rc5") that auto-increment with each commit.
- The `main` branch contains stable releases only.

## 📝 Contributing

Contributions are welcome! Please read our [Contributing Guidelines](CONTRIBUTING.md).

### Development Workflow

**Branch Strategy:**
- `main` - Stable releases only (protected)
- `develop` - Daily development work (default working branch)

**Release Candidate (RC) Automation:**
- Every commit to `develop` automatically creates an RC release
- RC versions increment automatically: `v3.24.0-rc1`, `v3.24.0-rc2`, etc.
- Docker images are built for both `amd64` and `arm64` architectures
- Local development shows dynamic RC versions that update with each commit

**Making Contributions:**
1. Fork the repository
2. Create a feature branch from `develop`
3. Make your changes
4. Test locally (version will show as RC automatically)
5. Submit a pull request to `develop`

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
1. **Check API Token Permissions (Most Common Issue):**
   ```bash
   # On your Proxmox host, verify token permissions:
   pveum user permissions <username>@pam!<tokenname>
   # Example: pveum user permissions pulse-monitor@pam!pulse
   ```
   - Look for `VM.Audit` permission at path `/`
   - If missing, you likely skipped the "Add Token Permission" step
   
2. **Verify Token Configuration:**
   - Token format must be: `username@realm!tokenname` (e.g., `pulse-monitor@pam!pulse`)
   - Path must be `/` (root), NOT `/access` or other paths
   - Propagate must be enabled (checked)
   - Role must be `PVEAuditor` or higher
   
3. **Test connectivity:** Can you ping your Proxmox host from where Pulse is running?

4. **Check Privilege Separation Setting:**
   - With privilege separation disabled: Ensure permissions are set on the USER only
   - With privilege separation enabled (default): Ensure permissions are set on BOTH USER and TOKEN
   - The easiest solution is to disable privilege separation and set permissions on the user only

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
    - **Update (v3.30.0+):** Pulse uses the bulk `/cluster/resources` endpoint, reducing API calls by up to 95% while maintaining 2-second polling
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
    - **PVE backups not showing:** This is usually a permission issue:
      - Run the web-based diagnostics: http://your-pulse-ip:7655/diagnostics.html
      - Check the "Recommendations" section for specific permission issues
      - Or manually check if your token has privilege separation: `pveum user token list user@realm --output-format json`
      - If `privsep` is 0, permissions only need to be set on the USER: `pveum acl modify /storage --users user@realm --roles PVEDatastoreAdmin`
      - If `privsep` is 1 (default), permissions must be set on BOTH USER and TOKEN: 
        ```
        pveum acl modify /storage --users user@realm --roles PVEDatastoreAdmin
        pveum acl modify /storage --tokens user@realm!tokenname --roles PVEDatastoreAdmin
        ```
      - See the [Storage Content Visibility](#required-permissions) section for full details.
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
