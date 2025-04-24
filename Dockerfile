# Use an official Node runtime as a parent image
# Choose a version compatible with your application (e.g., LTS version like 18 or 20)
FROM node:18-alpine

# Set the working directory in the container
WORKDIR /usr/src/app

# Copy package.json and package-lock.json for root dependencies
COPY package*.json ./

# Install root dependencies
RUN npm install

# Copy server package.json and package-lock.json
COPY server/package*.json ./server/

# Install server dependencies
RUN cd server && npm install

# Copy the rest of the application code
# Copy server code first
COPY server/ ./server/
# Copy public directory into src/public to match server path expectations
COPY src/public/ ./src/public/

# Application listens on port 7655 by default (as per .env.example)
EXPOSE 7655

# Define the command to run the app using the start script from root package.json
# This script should handle changing into the server directory
CMD [ "npm", "run", "start" ]