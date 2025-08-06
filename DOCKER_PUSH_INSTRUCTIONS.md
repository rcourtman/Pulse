# Docker Push Instructions for v4.0.6

The Docker image has been built locally but needs to be pushed from a machine with Docker Hub credentials.

## Option 1: Push from this machine
```bash
# Login to Docker Hub
sudo docker login -u rcourtman

# Push all tags
sudo docker push rcourtman/pulse:v4.0.6
sudo docker push rcourtman/pulse:4.0.6
sudo docker push rcourtman/pulse:4.0
sudo docker push rcourtman/pulse:4
sudo docker push rcourtman/pulse:latest
```

## Option 2: Build and push from docker-builder container (192.168.0.174)
```bash
ssh root@192.168.0.174
cd /root/Pulse
git pull
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t rcourtman/pulse:v4.0.6 \
  -t rcourtman/pulse:4.0.6 \
  -t rcourtman/pulse:4.0 \
  -t rcourtman/pulse:4 \
  -t rcourtman/pulse:latest \
  --push .
```

## Option 3: Build multi-arch locally with buildx
```bash
# Create buildx builder if not exists
docker buildx create --name multiarch --use

# Build and push
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t rcourtman/pulse:v4.0.6 \
  -t rcourtman/pulse:4.0.6 \
  -t rcourtman/pulse:4.0 \
  -t rcourtman/pulse:4 \
  -t rcourtman/pulse:latest \
  --push .
```

The v4.0.6 release fixes:
- Docker persistence issue (#253)
- Windows VM memory reporting with balloon drivers (#258)