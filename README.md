# <img src="public/logos/pulse-logo-256x256.png" alt="Pulse Logo" width="32" height="32" style="vertical-align: middle"> Pulse for Proxmox VE

A lightweight monitoring application for Proxmox VE that displays real-time status for VMs and containers via a simple web interface.

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/rcourtman)

## 📋 Table of Contents
- [Configuration](#️-configuration)
  - [Environment Variables](#environment-variables)
  - [Creating a Proxmox API Token](#creating-a-proxmox-api-token)
  - [Required Permissions](#required-permissions)
- [Installation](#-installation)
- [Running the Application](#-running-the-application)
- [Running the Application (Node.js)](#️-running-the-application-nodejs)
- [Running with Docker Compose](#-running-with-docker-compose)
- [Features](#-features)
- [System Requirements](#-system-requirements)
- [Contributing](#-contributing)
- [License](#-license)
- [Trademark Notice](#trademark-notice)
- [Support](#-support)

## 🛠️ Configuration

### Environment Variables

1.  **Copy Example File:** This application requires environment variables for configuration. Copy the example environment file from `server/.env.example` to `server/.env`.

    ```bash
    cp server/.env.example server/.env
    ```

2.  **Edit `.env`:** Open `server/.env` in a text editor and update the values for your Proxmox environment, including the Host, Token ID, and Token Secret obtained below.

    The following variables are available:
    - `PROXMOX_HOST`: URL of your Proxmox server (e.g., `https://your-proxmox-ip:8006`).
    - `PROXMOX_TOKEN_ID`: Your API Token ID (e.g., `user@pam!tokenid`).
    - `PROXMOX_TOKEN_SECRET`: Your API Token Secret.
    - `PROXMOX_ALLOW_SELF_SIGNED_CERTS`: (Optional) Set to `true` if your Proxmox server uses self-signed SSL certificates. Defaults to `false`.
    - `PORT`: (Optional) Port for the Pulse server to listen on. Defaults to `7655`.
    - `PROXMOX_USERNAME`, `PROXMOX_PASSWORD`, `PROXMOX_REALM`: (Optional) Fallback credentials if API token is not provided.

### Creating a Proxmox API Token

An API token is recommended for connecting Pulse to Proxmox.

1.  **Log in to the Proxmox web interface**

2.  **Create a dedicated user** (optional but recommended for security)
    *   Go to `Datacenter` → `Permissions` → `Users`.
    *   Click `Add`.
    *   Enter a `User name` (e.g., "pulse-monitor"), set Realm to `Proxmox VE authentication server`, set a password, and ensure `Enabled` is checked. Click `Add`.

3.  **Create an API token**
    *   Go to `Datacenter` → `Permissions` → `API Tokens`.
    *   Click `Add`.
    *   Select the `User` you created (e.g., "pulse-monitor@pam") or `root@pam`.
    *   Enter a `Token ID` (e.g., "pulse").
    *   Leave `Privilege Separation` checked (more secure).
    *   Click `Add`.
    *   **Important:** Copy the displayed `Secret` value immediately and store it securely. It will only be shown once.

4.  **Assign permissions**
    *   Go to `Datacenter` → `Permissions` → `Add` → `User Permission`.
    *   Path: `/`
    *   User: Select the user the token belongs to (e.g., "pulse-monitor@pam").
    *   Role: `PVEAuditor` (provides read-only access).
    *   Ensure `Propagate` is checked.
    *   Click `Add`.

5.  **Update your `server/.env` file** with the `Token ID` (which looks like `user@realm!tokenid`, e.g., `pulse-monitor@pam!pulse`) and the `Secret` you saved.

### Required Permissions

The `PVEAuditor` role is recommended as it provides the necessary read-only permissions for Pulse to monitor your Proxmox environment:
- `Datastore.Audit`
- `Permissions.Read` (implicitly included)
- `Pool.Audit`
- `Sys.Audit`
- `VM.Audit`

## 💾 Installation

Navigate to the project root directory and install the necessary Node.js dependencies.

```bash
# Install root dependencies
npm install
```

You also need to install dependencies for the server component:

```bash
# Install server dependencies
cd server
npm install
cd ..
```

## ▶️ Running the Application (Node.js)

These instructions are for running the application directly using Node.js.

### Development Mode

To run the application in development mode (useful for testing changes):

```bash
npm run dev
```
This command starts the server, typically using `nodemon` or similar for automatic restarts on file changes. Check the terminal output for the URL (e.g., `http://localhost:7655`).

### Production Mode

To run the application normally:

```bash
npm run start
```
This command starts the server using `node`. Access the application via the configured host and port.

## 🐳 Running with Docker Compose

Using Docker Compose is the recommended way to run the application in a containerized environment.

**Prerequisites:**
- Docker ([Install Docker](https://docs.docker.com/engine/install/))
- Docker Compose ([Install Docker Compose](https://docs.docker.com/compose/install/))

**Steps:**

1.  **Configure Environment:** Ensure you have created and configured your `server/.env` file as described in the [Environment Variables](#environment-variables) section above.

2.  **Run:** Navigate to the project root directory in your terminal and run:
    ```bash
    docker compose up -d
    ```
    - This command will download the pre-built `rcourtman/pulse:latest` image from Docker Hub (if not already present) and start the container.
    - `-d`: Runs the container in detached mode (in the background).

3.  **Access:** The application should now be running. Access it via `http://<your-host-ip>:7655` (or the host port you mapped in `docker-compose.yml`).

**Stopping the Application:**

To stop the container(s) defined in the `docker-compose.yml` file, run:

```bash
docker compose down
```

*Note: If you modify the `server/.env` file after the container is already running, you may need to restart the container for the changes to take effect. You can do this by running `docker compose down` followed by `docker compose up -d`, or by using `docker compose up -d --force-recreate`.*

## ✨ Features

- Lightweight monitoring for Proxmox VE nodes.
- Displays real-time status for VMs and Containers.
- Simple web interface.

## 💻 System Requirements

- **Node.js** (for local development/running): Version 16.x or higher recommended.
- **Docker & Docker Compose** (for containerized deployment): Recent versions recommended.
- **Network**: Connectivity between the Pulse server and your Proxmox server(s).

## 👥 Contributing

Contributions are welcome! Please follow standard fork-and-pull-request workflow. Refer to the main repository's contributing guidelines if available.

1.  **Fork the repository**
2.  **Create a feature branch**: `