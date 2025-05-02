# <img src="src/public/logos/pulse-logo-256x256.png" alt="Pulse Logo" width="32" height="32" style="vertical-align: middle"> Pulse for Proxmox VE

[![GitHub release (latest by date)](https://img.shields.io/github/v/release/rcourtman/Pulse)](https://github.com/rcourtman/Pulse/releases/latest)
[![License](https://img.shields.io/github/license/rcourtman/Pulse)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/rcourtman/pulse)](https://hub.docker.com/r/rcourtman/pulse)

A lightweight monitoring application for Proxmox VE that displays real-time status for VMs and containers via a simple web interface.

![Pulse Dashboard](docs/images/01-dashboard.png)

### 📸 Screenshots

<details>
<summary><strong>Click to view other UI tabs</strong></summary>

**Nodes Tab:**
![Nodes View](docs/images/02-node-view.png)

**Main Tab (Filtered for VMs):**
![VM Filtered View](docs/images/03-vm-container-view.png)

**PBS Tab:**
![PBS View](docs/images/04-pbs-view.png)

**Backups Tab:**
![Backups View](docs/images/05-backups-view.png)

</details>

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/rcourtman)

## 📋 Table of Contents
- [Quick Start (Docker Compose)](#-quick-start-docker-compose)
- [Configuration](#️-configuration)
  - [Environment Variables](#environment-variables)
  - [Creating a Proxmox API Token](#creating-a-proxmox-api-token)
  - [Creating a Proxmox Backup Server API Token](#creating-a-proxmox-backup-server-api-token)
  - [Required Permissions](#required-permissions)
- [Deployment](#-deployment)
  - [Running with Docker Compose](#-running-with-docker-compose)
  - [Running with LXC Installation Script](#-running-with-lxc-installation-script)
  - [Running with Node.js (Development)](#️-running-the-application-nodejs-development)
- [Features](#-features)
- [System Requirements](#-system-requirements)
- [Contributing](#-contributing)
- [Privacy](#-privacy)
- [License](#-license)
- [Trademark Notice](#trademark-notice)
- [Support](#-support)
- [Troubleshooting](#-troubleshooting)

## 🚀 Quick Start (Docker Compose)

This is the fastest way to get Pulse running.

1.  **Get Files:** Clone the repository (`git clone https://github.com/rcourtman/Pulse.git && cd Pulse`) or download `docker-compose.yml` and `.env.example` manually.
2.  **Copy `.env`:** `cp .env.example .env`
3.  **Edit `.env`:** Fill in your primary Proxmox API details (`PROXMOX_HOST`, `PROXMOX_TOKEN_ID`, `PROXMOX_TOKEN_SECRET`). See [Creating a Proxmox API Token](#creating-a-proxmox-api-token) if you don't have one.
4.  **Run:** `docker compose up -d`
5.  **Access:** Open your browser to `http://<your-host-ip>:7655`.

## 🛠️ Configuration

### Environment Variables

Pulse is configured using environment variables, typically set in a `.env` file in the project root.

1.  **Copy Example File:** If you haven't already, copy the example file:
    ```bash
    cp .env.example .env
    ```
2.  **Edit `.env`:** Open `.env` in a text editor and update the values for your environment(s).

#### Proxmox VE (Primary Environment)

These are the minimum required variables:
-   `PROXMOX_HOST`: URL of your Proxmox server (e.g., `https://192.168.1.10:8006`).
-   `PROXMOX_TOKEN_ID`: Your API Token ID (e.g., `user@pam!tokenid`).
-   `PROXMOX_TOKEN_SECRET`: Your API Token Secret.

Optional variables:
-   `PROXMOX_NODE_NAME`: A display name for this endpoint in the UI (defaults to `PROXMOX_HOST`).
-   `PROXMOX_ALLOW_SELF_SIGNED_CERTS`: Set to `true` if your Proxmox server uses self-signed SSL certificates. Defaults to `false`.
-   `PORT`: Port for the Pulse server to listen on. Defaults to `7655`.
-   *(Username/Password fallback exists but API Token is strongly recommended)*

***Note:** For a Proxmox cluster, you only need to provide connection details for **one** node. Pulse automatically discovers other cluster members.*


#### Multiple Proxmox Environments (Optional)

To monitor separate Proxmox environments (e.g., different clusters, sites) in one Pulse instance, add numbered variables:

-   `PROXMOX_HOST_2`, `PROXMOX_TOKEN_ID_2`, `PROXMOX_TOKEN_SECRET_2`
-   `PROXMOX_HOST_3`, `PROXMOX_TOKEN_ID_3`, `PROXMOX_TOKEN_SECRET_3`
-   ...and so on.

Optional numbered variables also exist (e.g., `PROXMOX_ALLOW_SELF_SIGNED_CERTS_2`, `PROXMOX_NODE_NAME_2`).

#### Proxmox Backup Server (PBS) (Optional)

To monitor a PBS instance:

-   `PBS_HOST`: URL of your PBS server (e.g., `https://192.168.1.11:8007`).
-   `PBS_TOKEN_ID`: Your PBS API Token ID (e.g., `user@pbs!tokenid`). See [Creating a Proxmox Backup Server API Token](#creating-a-proxmox-backup-server-api-token).
-   `PBS_TOKEN_SECRET`: Your PBS API Token Secret.
-   `PBS_NODE_NAME`: **Important!** The internal hostname of your PBS server (e.g., `pbs-server-01`). This is usually required for API token auth because the token might lack permission to auto-discover the node name. See details below.
-   `PBS_ALLOW_SELF_SIGNED_CERTS`: Set to `true` for self-signed certificates. Defaults to `false`.
-   `PBS_PORT`: PBS API port. Defaults to `8007`.

*Note: Currently, only one PBS instance can be configured.*

<details>
<summary><strong>Why <code>PBS_NODE_NAME</code> is Required (Click to Expand)</strong></summary>

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
4.  **Assign permissions:**
    *   Go to `Configuration` → `Access Control` → `Permissions`.
    *   Click `Add` → `API Token Permission`.
    *   Path: `/` (root level).
    *   API Token: Select the token (e.g., "pulse-monitor@pbs!pulse").
    *   Role: `Audit`.
    *   Check `Propagate`. Click `Add`.
    *   *Note: The `Audit` role at root path (`/`) is needed for read-only access.*
5.  **Update `.env`:** Set `PBS_TOKEN_ID` (e.g., `pulse-monitor@pbs!pulse`) and `PBS_TOKEN_SECRET`.

</details>

### Required Permissions

-   **Proxmox VE:** The `PVEAuditor` role assigned at path `/` with `Propagate` enabled is recommended.
    <details>
    <summary>Permissions included in PVEAuditor (Click to Expand)</summary>
    - `Datastore.Audit`
    - `Permissions.Read` (implicitly included)
    - `Pool.Audit`
    - `Sys.Audit`
    - `VM.Audit`
    </details>
-   **Proxmox Backup Server:** The `Audit` role assigned at path `/` with `Propagate` enabled is recommended.

## 🚀 Deployment

Choose one of the following methods to deploy Pulse.

### Running with Docker Compose

Using Docker Compose is the recommended way for most users.

**Prerequisites:**
- Docker ([Install Docker](https://docs.docker.com/engine/install/))
- Docker Compose ([Install Docker Compose](https://docs.docker.com/compose/install/))

**Steps:**

1.  **Configure Environment:** Ensure your `.env` file is created and configured as described in [Configuration](#️-configuration).
2.  **Run:** In the project root directory, run:
    ```bash
    docker compose up -d
    ```