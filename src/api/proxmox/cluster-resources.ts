import { ProxmoxClient } from './index';
import { ProxmoxVM, ProxmoxContainer } from '../../types';
import config from '../../config';

/**
 * Generate a unique ID for a VM or container
 * @param type The type of guest ('qemu' or 'lxc')
 * @param vmid The VM ID
 * @param nodeName The node name
 * @returns A unique ID string
 */
function generateClusterGuestId(type: string, vmid: number): string {
  // In cluster mode, use the cluster name for the ID
  return type === 'qemu'
    ? `${config.clusterName}-vm-${vmid}`
    : `${config.clusterName}-ct-${vmid}`;
}

/**
 * Get all cluster resources (VMs and containers) from a single API call
 * This is much more efficient for clusters than querying each node separately
 */
export async function getClusterResources(this: ProxmoxClient): Promise<{
  vms: ProxmoxVM[];
  containers: ProxmoxContainer[];
  nodes: string[];
}> {
  try {
    if (!this.client) {
      throw new Error('Client is not initialized');
    }

    this.logger.info('Fetching cluster resources using /cluster/resources endpoint');

    // Fetch all resources from the cluster endpoint
    const response = await this.client.get('/cluster/resources');
    const resources = response.data.data || [];

    // Split the resources into VMs and containers
    const vms: ProxmoxVM[] = [];
    const containers: ProxmoxContainer[] = [];
    
    // Keep track of all unique node names
    const nodeSet = new Set<string>();
    
    // Process each resource
    for (const resource of resources) {
      // Collect all unique node names
      if (resource.node) {
        nodeSet.add(resource.node);
      }
      
      // Only process VM and container resources
      if (resource.type === 'qemu') {
        try {
          // Get detailed resource usage for this VM if we have node and VMID
          let resourceData: any = {};
          
          try {
            if (resource.node && resource.vmid) {
              const response = await this.client.get(`/nodes/${resource.node}/${resource.type}/${resource.vmid}/status/current`);
              resourceData = response.data.data || {};
            }
          } catch (error) {
            this.logger.warn(`Could not fetch detailed VM resource data for ${resource.vmid} on ${resource.node}`, { error });
          }
          
          // Merge the resource data with the basic VM info
          const vm: ProxmoxVM = {
            id: generateClusterGuestId('qemu', resource.vmid),
            name: resource.name,
            status: resource.status,
            node: resource.node,  // Use the actual node name from the resource
            vmid: resource.vmid,
            cpus: resource.maxcpu || 1,
            cpu: resource.cpu,
            memory: resource.mem || 0,
            maxmem: resource.maxmem || 0,
            disk: resource.disk || 0,
            maxdisk: resource.maxdisk || 0,
            uptime: resource.uptime || 0,
            netin: resource.netin || 0,
            netout: resource.netout || 0,
            diskread: resource.diskread || 0,
            diskwrite: resource.diskwrite || 0,
            template: resource.template === 1,
            type: 'qemu'
          };
          
          vms.push(vm);
        } catch (error) {
          this.logger.error(`Error processing VM resource for ${resource.vmid}`, { error });
        }
      } else if (resource.type === 'lxc') {
        try {
          // Get detailed resource usage for this container if we have node and VMID
          let resourceData: any = {};
          
          try {
            if (resource.node && resource.vmid) {
              const response = await this.client.get(`/nodes/${resource.node}/${resource.type}/${resource.vmid}/status/current`);
              resourceData = response.data.data || {};
            }
          } catch (error) {
            this.logger.warn(`Could not fetch detailed container resource data for ${resource.vmid} on ${resource.node}`, { error });
          }
          
          // Merge the resource data with the basic container info
          const container: ProxmoxContainer = {
            id: generateClusterGuestId('lxc', resource.vmid),
            name: resource.name,
            status: resource.status,
            node: resource.node,  // Use the actual node name from the resource
            vmid: resource.vmid,
            cpus: resource.maxcpu || 1,
            cpu: resource.cpu,
            memory: resource.mem || 0,
            maxmem: resource.maxmem || 0,
            disk: resource.disk || 0,
            maxdisk: resource.maxdisk || 0,
            uptime: resource.uptime || 0,
            netin: resource.netin || 0,
            netout: resource.netout || 0,
            diskread: resource.diskread || 0,
            diskwrite: resource.diskwrite || 0,
            template: resource.template === 1,
            type: 'lxc'
          };
          
          containers.push(container);
        } catch (error) {
          this.logger.error(`Error processing container resource for ${resource.vmid}`, { error });
        }
      }
    }

    this.logger.info(`Processed ${vms.length} VMs and ${containers.length} containers from cluster resources`);
    
    return { vms, containers, nodes: Array.from(nodeSet) };
  } catch (error) {
    this.logger.error('Error getting cluster resources', { error });
    return { vms: [], containers: [], nodes: [] };
  }
} 