# ---- Dependencies Stage ----
FROM node:20-alpine AS deps
WORKDIR /usr/src/app
COPY package*.json ./
RUN npm ci --omit=dev

# ---- Builder Stage ----
FROM node:20-alpine AS builder

WORKDIR /usr/src/app

# Copy package files and install ALL dependencies
COPY package*.json ./
RUN npm ci

# Copy only the files needed for building
COPY src/ ./src/
COPY server/ ./server/

# Build the production CSS
RUN npm run build:css

# ---- Runner Stage ----
FROM node:20-alpine

WORKDIR /usr/src/app

# Use existing node user (uid:gid 1000:1000) instead of system service accounts
# The node:18-alpine image already has a 'node' user with uid:gid 1000:1000

# Copy production dependencies from deps stage
COPY --from=deps --chown=node:node /usr/src/app/node_modules ./node_modules

# Copy built assets from builder
COPY --from=builder --chown=node:node /usr/src/app/src/public ./src/public

# Copy server code and package.json
COPY --chown=node:node server/ ./server/
COPY --chown=node:node package.json ./

# Create config directory for persistent volume mount and data directory
RUN mkdir -p /usr/src/app/config /usr/src/app/data

# Ensure correct ownership of directories
RUN chown -R node:node /usr/src/app/config /usr/src/app/data

# Switch to non-root user
USER node

# Set environment variable to indicate Docker deployment
ENV DOCKER_DEPLOYMENT=true

# Expose port
EXPOSE 7655

# Add health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=40s --retries=3 \
  CMD node -e "require('http').get('http://localhost:7655/api/health/healthz', (res) => process.exit(res.statusCode === 200 ? 0 : 1))"

# Run the application using the start script
CMD [ "npm", "run", "start" ]