# Build stage for the client
FROM node:18-alpine AS client-builder
WORKDIR /app
COPY package*.json ./
COPY client/package*.json ./client/
RUN npm ci --workspace=client --include-workspace-root

COPY client ./client
RUN npm run build:client

# Build stage for the server
FROM node:18-alpine AS server-builder
WORKDIR /app
COPY package*.json ./
COPY server/package*.json ./server/
RUN npm ci --workspace=server --include-workspace-root

COPY server ./server
# No build step for server, but we could add one if needed

# Production stage
FROM node:18-alpine
WORKDIR /app

# Create a non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Copy built client files
COPY --from=client-builder /app/client/dist ./client/dist

# Copy server files
COPY --from=server-builder /app/package*.json ./
COPY --from=server-builder /app/server/package*.json ./server/
RUN npm ci --workspace=server --include-workspace-root --production

COPY --from=server-builder /app/server ./server

# Expose the port
EXPOSE 3000

# Set environment variables
ENV NODE_ENV=production

# Start the server
CMD ["npm", "start"] 