# Pulse - Proxmox Monitoring Dashboard

![License](https://img.shields.io/github/license/yourusername/pulse)
![GitHub stars](https://img.shields.io/github/stars/yourusername/pulse?style=social)

Pulse is a lightweight, real-time monitoring dashboard for Proxmox servers. It provides a clean, intuitive interface to monitor CPU, memory, and network usage across multiple Proxmox servers, VMs, and containers.

## Features

- **Real-time Monitoring**: View live metrics with 2-second refresh rate
- **Multi-server Support**: Monitor multiple Proxmox servers from a single dashboard
- **Interactive Filtering**: Use sliders to filter resources based on metric thresholds
- **Sorting**: Sort resources by CPU, memory, or network usage
- **Search**: Quickly find resources with the search functionality
- **Environment-based Configuration**: Configure Proxmox nodes via environment variables
- **Secure**: Credentials are stored securely in environment variables
- **Lightweight**: Optimized for performance with minimal resource usage
- **Containerized**: Easy deployment with Docker

## Screenshots

![Dashboard](../client/src/assets/dashboard-preview.png)

## Technology Stack

### Frontend
- React
- React Router
- Zustand (State Management)
- Tailwind CSS
- Socket.io Client
- Framer Motion

### Backend
- Node.js
- Express
- Socket.io
- Axios

## Installation

### Using Docker

```bash
docker pull ghcr.io/yourusername/pulse:latest
docker run -p 3000:3000 \
  -e PROXMOX_NODE_1_NAME="Proxmox Node 1" \
  -e PROXMOX_NODE_1_HOST="https://192.168.0.132:8006" \
  -e PROXMOX_NODE_1_TOKEN_ID="root@pam!pulse" \
  -e PROXMOX_NODE_1_TOKEN_SECRET="your-token-secret" \
  ghcr.io/yourusername/pulse:latest
```

Then open your browser and navigate to `http://localhost:3000`.

### Development Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/pulse.git
   cd pulse
   ```

2. Set up environment variables:
   ```bash
   # Copy the example environment file
   cp server/.env.example server/.env
   
   # Edit the .env file with your Proxmox node details
   nano server/.env
   ```

3. Run the start script:
   ```bash
   ./start.sh
   ```

4. Open your browser and navigate to `http://localhost:5173`.

## Environment Variables

Configure your Proxmox nodes using environment variables in the `server/.env` file:

```
# Node 1
PROXMOX_NODE_1_NAME=Proxmox Node 1
PROXMOX_NODE_1_HOST=https://192.168.0.132:8006
PROXMOX_NODE_1_TOKEN_ID=root@pam!pulse
PROXMOX_NODE_1_TOKEN_SECRET=your-token-secret

# Node 2
PROXMOX_NODE_2_NAME=Proxmox Node 2
PROXMOX_NODE_2_HOST=https://192.168.0.141:8006
PROXMOX_NODE_2_TOKEN_ID=root@pam!pulse
PROXMOX_NODE_2_TOKEN_SECRET=your-token-secret
```

You can add as many nodes as needed by incrementing the number:

```
PROXMOX_NODE_3_NAME=Proxmox Node 3
PROXMOX_NODE_3_HOST=https://192.168.0.150:8006
PROXMOX_NODE_3_TOKEN_ID=root@pam!pulse
PROXMOX_NODE_3_TOKEN_SECRET=your-token-secret
```

## Proxmox API Token Setup

To use Pulse, you need to create an API token in your Proxmox server:

1. Log in to your Proxmox web interface
2. Navigate to Datacenter → Permissions → API Tokens
3. Click on "Add" to create a new API token
4. Enter a user (e.g., root@pam) and a token ID (e.g., pulse)
5. Uncheck "Privilege Separation"
6. Click "Add" to create the token
7. Copy both the token ID and secret value (the secret will only be shown once)
8. Assign the PVEAuditor role to the token (Permissions → Add → API Token Permission)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.

## Acknowledgments

- [Proxmox](https://www.proxmox.com/) for their amazing virtualization platform
- [React](https://reactjs.org/) for the frontend framework
- [Node.js](https://nodejs.org/) for the backend runtime
- All the open-source libraries that made this project possible 