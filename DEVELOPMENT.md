# Development Guide

This guide provides instructions for setting up and running the Pulse application directly using Node.js, typically for development or contribution purposes. Users looking to deploy Pulse should refer to the main [README.md](README.md) for Docker or LXC instructions.

## 💾 Installation (from Source)

If you intend to run the application directly from source or contribute to development, you need to install dependencies.

1.  **Clone the Repository:**
    ```bash
    git clone https://github.com/rcourtman/Pulse.git
    cd Pulse
    ```

2.  **Install Dependencies:** Navigate to the project root directory and install all necessary Node.js dependencies:
    ```bash
    npm install
    ```

    **Note:** The project uses a unified dependency structure - all dependencies are managed from the root `package.json`. There's no separate server directory with its own dependencies.

3.  **Build CSS:** Compile the Tailwind CSS styles:
    ```bash
    npm run build:css
    ```

## ▶️ Running the Application (Node.js)

These instructions assume you have completed the installation steps above.

### Development Mode

To run the application in development mode with hot-reloading for both server and CSS:

```bash
npm run dev
```

This command:
- Starts the server with `NODE_ENV=development` and automatic dotenv loading
- Watches for CSS changes and rebuilds Tailwind styles automatically
- Provides live-reload functionality for faster development

The application will be accessible at `http://localhost:7655` (or the port configured in your `.env` file).

### Production Mode (Direct Node Execution)

To run the application using a standard `node` process, similar to how it runs in production:

```bash
npm run start
```

This command starts the server using `node server/index.js`. Access the application via the configured host and port (defaulting to `http://localhost:7655`).

### Individual Development Commands

For more granular control during development:

```bash
# Run only the server in development mode
npm run dev:server

# Watch and rebuild CSS only
npm run dev:css

# Build CSS for production (minified)
npm run build:css
```

## 🔧 Development Workflow

### Branch Strategy
- **`main`** - Stable releases only (protected branch)
- **`develop`** - Daily development work (default working branch)
- **Feature branches** - Created from `develop` for specific features

### Release Candidates
- Every commit to `develop` automatically creates an RC release
- RC versions increment automatically: `v3.28.0-rc1`, `v3.28.0-rc2`, etc.
- Local development shows dynamic RC versions that update with each commit

### Making Changes
1. Work on the `develop` branch (stay here for all development)
2. Make your changes and test locally
3. Commit and push to trigger automatic RC releases
4. For stable releases, create a PR from `develop` to `main`

## 📁 Project Structure

```
pulse/
├── src/public/          # Frontend application
│   ├── js/ui/          # Modular Vue.js components
│   ├── css/            # Source styles
│   └── output.css      # Compiled Tailwind CSS
├── server/             # Backend Node.js application
│   ├── index.js        # Main server entry point
│   ├── *.js           # Modular server components
│   └── routes/        # API route handlers
├── scripts/           # Installation and utility scripts
├── docs/             # Technical documentation
└── .github/          # GitHub workflows and templates
```

## ⚙️ Configuration

Create a `.env` file in the project root for local development. See `.env.example` for available options.

**Required for development:**
```env
PROXMOX_HOST=https://your-proxmox-host:8006
PROXMOX_TOKEN_ID=your-token-id
PROXMOX_TOKEN_SECRET=your-token-secret
```

**Optional development settings:**
```env
NODE_ENV=development
PORT=7655
DEBUG=pulse:*
``` 