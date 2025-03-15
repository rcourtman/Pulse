# Testing the Cluster Resources Feature with Docker

Hi Tukamok, I've built a special Docker image with the cluster resources feature for you to test. This way, you don't need to check out the code - you can just run the container directly.

## Running the Test Container

1. Create a new file named `docker-compose.cluster-test.yml` with the following content:

```yaml
services:
  app:
    image: rcourtman/pulse:cluster-resources-test
    container_name: pulse-cluster-test
    ports:
      - "7654:7654"
    volumes:
      - ./logs:/app/logs
    environment:
      - NODE_ENV=production
      - LOG_LEVEL=debug
      - PROXMOX_NODE_1_NAME=YOUR_NODE_NAME
      - PROXMOX_NODE_1_HOST=YOUR_NODE_HOST
      - PROXMOX_NODE_1_TOKEN_ID=YOUR_TOKEN_ID
      - PROXMOX_NODE_1_TOKEN_SECRET=YOUR_TOKEN_SECRET
      # SSL verification settings (adjust as needed)
      - IGNORE_SSL_ERRORS=true
      - NODE_TLS_REJECT_UNAUTHORIZED=0
    restart: unless-stopped
```

2. Replace the `YOUR_NODE_*` placeholders with your actual Proxmox node details (just configure one node from your cluster)

3. Run the container:
```bash
docker compose -f docker-compose.cluster-test.yml up -d
```

4. Check the logs:
```bash
docker logs -f pulse-cluster-test
```

## What to Look For:

The cluster resources feature is now **enabled by default** - no need to set any additional environment variables. When you run the container:

1. You should see VMs and containers from **all nodes** in your cluster, not just the one you configured.
2. Resources should be properly organized by node.
3. In the logs, you should see messages about using the cluster resources endpoint.

## Reporting Back:

Please let me know:
1. Were all VMs/containers from all cluster nodes visible?
2. Did the dashboard correctly show which node each VM/container belongs to?
3. Was there any noticeable performance improvement?
4. Did you encounter any errors or issues?

## Need to Disable the Feature?

If you want to disable the feature for testing:

```yaml
environment:
  # Other env vars...
  - PROXMOX_USE_CLUSTER_RESOURCES=false
```

Thanks for your help testing this!
