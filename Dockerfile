# ---- Builder Stage ----
FROM node:18-alpine AS builder

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
FROM node:18-alpine

WORKDIR /usr/src/app

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

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

# Ensure correct ownership of application files
# Use /usr/src/app to cover everything copied
RUN chown -R appuser:appgroup /usr/src/app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 7655

# Run the application using the start script
CMD [ "npm", "run", "start" ]