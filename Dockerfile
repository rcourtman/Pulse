# ---- Builder Stage ----
FROM node:20-alpine AS builder

WORKDIR /usr/src/app

# Install necessary build tools (if any, e.g., python, make for some native deps)
# RUN apk add --no-cache ...

# Copy only necessary package files first
COPY package*.json ./

# Install ALL dependencies (including dev needed for build)
# Using npm ci for faster, more reliable builds in CI/CD
RUN npm ci

# Copy the rest of the application code
# Important: Copy . before running build commands
COPY . .

# Build the production CSS
RUN npm run build:css

# Prune devDependencies after build
RUN npm prune --production

# ---- Runner Stage ----
FROM node:20-alpine

WORKDIR /usr/src/app

# Use existing node user (uid:gid 1000:1000) instead of system service accounts
# The node:18-alpine image already has a 'node' user with uid:gid 1000:1000

# Copy necessary files from builder stage
# Copy node_modules first (can be large)
COPY --from=builder /usr/src/app/node_modules ./node_modules
# Copy built assets
COPY --from=builder /usr/src/app/src/public ./src/public
# Copy server code
COPY --from=builder /usr/src/app/server ./server
# Copy root package.json needed for npm start and potentially other metadata
COPY --from=builder /usr/src/app/package.json ./
# Optionally copy other root files if needed by the application (e.g., .env.example, README)
# COPY --from=builder /usr/src/app/.env.example ./

# Create config directory for persistent volume mount and data directory
RUN mkdir -p /usr/src/app/config /usr/src/app/data

# Ensure correct ownership of application files
# Use /usr/src/app to cover everything copied
RUN chown -R node:node /usr/src/app

# Switch to non-root user
USER node

# Set environment variable to indicate Docker deployment
ENV DOCKER_DEPLOYMENT=true

# Expose port
EXPOSE 7655

# Run the application using the start script
CMD [ "npm", "run", "start" ]