# Pulse - Proxmox Monitoring Dashboard

Pulse is a lightweight, real-time monitoring dashboard for Proxmox servers. It provides a clean, intuitive interface to monitor CPU, memory, and network usage across multiple Proxmox servers, VMs, and containers.

![Pulse Dashboard](./client/src/assets/dashboard-preview.png)

## Features

- **Real-time Monitoring**: View live metrics with 2-second refresh rate
- **Multi-server Support**: Monitor multiple Proxmox servers from a single dashboard
- **Interactive Filtering**: Use sliders to filter resources based on metric thresholds
- **Sorting**: Sort resources by CPU, memory, or network usage
- **Search**: Quickly find resources with the search functionality
- **Guided Setup**: Onboarding wizard to help set up Proxmox API tokens
- **Secure**: Credentials are encrypted and stored locally
- **Lightweight**: Optimized for performance with minimal resource usage
- **Containerized**: Easy deployment with Docker

## Quick Start

### Using Docker

```bash
docker pull ghcr.io/yourusername/pulse:latest
docker run -p 3000:3000 ghcr.io/yourusername/pulse:latest
```

Then open your browser and navigate to `http://localhost:3000`.

### Development Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/pulse.git
   cd pulse
   ```

2. Install dependencies and start the application:
   ```bash
   # The start script will install dependencies and start the application
   ./start.sh
   ```

3. Open your browser and navigate to `http://localhost:5173`.

### Production Deployment

For production deployment, you can use Docker:

```bash
# Build the Docker image
docker build -t pulse .

# Run the container
docker run -p 3000:3000 pulse
```

Or build and run manually:

```bash
# Build the client
cd client && npm run build

# Start the server (which will serve the client build)
cd ../server && npm run start
```

Then open your browser and navigate to `http://localhost:3000`.

## Configuration

Pulse stores your Proxmox server configurations locally in your browser's localStorage. All sensitive information is encrypted.

### Environment Variables

The server can be configured using environment variables:

- `PORT`: The port the server will listen on (default: 3000)
- `NODE_ENV`: The environment mode (development or production)
- `CLIENT_URL`: The URL of the client (for CORS)

Proxmox nodes can be pre-configured using environment variables:

```
PROXMOX_NODE_1_NAME=Proxmox Node 1
PROXMOX_NODE_1_HOST=https://192.168.0.132:8006
PROXMOX_NODE_1_TOKEN_ID=root@pam!pulse
PROXMOX_NODE_1_TOKEN_SECRET=your-token-secret

PROXMOX_NODE_2_NAME=Proxmox Node 2
PROXMOX_NODE_2_HOST=https://192.168.0.141:8006
PROXMOX_NODE_2_TOKEN_ID=root@pam!pulse
PROXMOX_NODE_2_TOKEN_SECRET=your-token-secret
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Proxmox](https://www.proxmox.com/) for their amazing virtualization platform
- [React](https://reactjs.org/) for the frontend framework
- [Node.js](https://nodejs.org/) for the backend runtime
- All the open-source libraries that made this project possible 