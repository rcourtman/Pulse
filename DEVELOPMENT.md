# Development Guide

This guide provides instructions for setting up and running the Pulse application directly using Node.js, typically for development or contribution purposes. Users looking to deploy Pulse should refer to the main [README.md](README.md) for Docker or LXC instructions.

## 💾 Installation (from Source)

If you intend to run the application directly from source or contribute to development, you need to install dependencies.

1.  **Clone the Repository:**
    ```bash
    git clone https://github.com/rcourtman/Pulse.git
    cd Pulse
    ```

2.  **Install Root Dependencies:** Navigate to the project root directory and install the necessary Node.js dependencies.
    ```bash
    # Install root dependencies
    npm install
    ```

3.  **Install Server Dependencies:** You also need to install dependencies specifically for the server component:
    ```bash
    # Install server dependencies
    cd server
    npm install
    cd ..
    ```

## ▶️ Running the Application (Node.js)

These instructions assume you have completed the installation steps above.

### Development Mode

To run the application in development mode, which typically enables features like hot-reloading for easier testing of changes:

```bash
npm run dev
```
This command starts the server (often using `nodemon` or a similar tool) which monitors for file changes and automatically restarts. Check the terminal output for the URL where the application is accessible (e.g., `http://localhost:7655`).

### Production Mode (Direct Node Execution)

To run the application using a standard `node` process, similar to how it might run in production if not containerized:

```bash
npm run start
```
This command starts the server using `node`. Access the application via the configured host and port (defaulting to `http://localhost:7655`).

**Note:** Ensure your `.env` file is correctly configured in the project root directory before running either command. 