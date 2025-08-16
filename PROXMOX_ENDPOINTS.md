# Proxmox API Endpoint Documentation

## Update Frequencies and Use Cases

Based on empirical testing against Proxmox VE, here's what each endpoint provides:

### Node Metrics

| Endpoint | Update Frequency | Use Case | Data Freshness |
|----------|-----------------|----------|----------------|
| `/nodes` | ~10 seconds | Node list, basic status | Cached/aggregated |
| `/nodes/{node}/status` | **1 second** | Real-time node metrics | Real-time |
| `/cluster/resources?type=node` | ~10 seconds | Cluster overview | Cached |

### VM/Container Metrics

| Endpoint | Update Frequency | Use Case | Data Freshness |
|----------|-----------------|----------|----------------|
| `/nodes/{node}/qemu` | On state change | VM list | Current |
| `/nodes/{node}/qemu/{vmid}/status/current` | **1 second** | Real-time VM metrics | Real-time |
| `/nodes/{node}/lxc` | On state change | Container list | Current |
| `/nodes/{node}/lxc/{vmid}/status/current` | **1 second** | Real-time container metrics | Real-time |

### Storage & Backup

| Endpoint | Update Frequency | Use Case | Data Freshness |
|----------|-----------------|----------|----------------|
| `/nodes/{node}/storage` | ~30 seconds | Storage overview | Cached |
| `/nodes/{node}/storage/{storage}/content` | On change | Backup listings | Current |
| `/nodes/{node}/tasks` | On change | Task status | Current |

## Key Findings

1. **Real-time endpoints** (`/status` and `/status/current`) update every second
2. **List endpoints** (`/nodes`, `/qemu`, `/lxc`) are cached/aggregated
3. **pvestatd** updates different endpoints at different rates
4. The commonly cited "10 second update interval" only applies to aggregated endpoints

## Recommended Polling Strategy

For real-time monitoring:
- **Node metrics**: Poll `/nodes/{node}/status` every 1-2 seconds
- **VM/Container metrics**: Poll `/status/current` endpoints every 1-2 seconds
- **Storage**: Poll every 30-60 seconds (changes less frequently)
- **Backup tasks**: Poll every 30-60 seconds or on-demand

## Test Results

### Test 1: /nodes endpoint
- Update interval: ~10 seconds
- Shows aggregated data across cluster

### Test 2: /nodes/{node}/status endpoint  
- Update interval: **1 second**
- Provides real-time CPU, memory, disk, uptime
- This is the endpoint to use for live monitoring

### Test 3: /cluster/resources endpoint
- Update interval: ~10 seconds  
- Similar to /nodes but includes VMs/containers

## Implementation Notes

Current Pulse implementation uses:
- `GetNodes()` - Uses `/nodes` (slow, cached)
- `GetNodeStatus()` - Uses `/nodes/{node}/status` (real-time)

We should prioritize GetNodeStatus() data over GetNodes() data for metrics that need to be real-time.