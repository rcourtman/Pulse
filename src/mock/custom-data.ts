/**
 * Custom Mock Data for Screenshots
 * 
 * This file contains custom mock data for generating screenshots.
 * It provides a consistent set of data with different guests for each node.
 * The data is designed to mimic typical Proxmox deployments with realistic naming and resource allocation.
 * Each node has a maximum of 10 guests to prevent overloading the UI.
 * 
 * !!!!! IMPORTANT !!!!!
 * The mock client and server rely on each node having EXACTLY 10 guests.
 * When editing this file, make sure each node maintains exactly 10 guests (including shared ones).
 * Otherwise, it will cause UI flickering and inconsistent behavior.
 */

/**
 * Interface for custom guest data
 */
export interface CustomGuest {
  id: number;
  name: string;
  type: 'vm' | 'ct';
  status: 'running' | 'stopped' | 'paused';
  cpu: number;
  memory: number;
  disk: { used: number; total: number; };
  
  // Optional properties for enhanced mock data
  cpus?: number;
  memoryMB?: number;
  diskGB?: number;
  shared?: boolean;
  primaryNode?: string;
  node?: string;
}

/**
 * Interface for custom node data
 */
export interface CustomNode {
  id: string;
  name: string;
  status: 'online' | 'offline';
  cpu: { usage: number; cores: number; };
  memory: { used: number; total: number; };
  guests: CustomGuest[];
}

/**
 * Custom mock data structure
 */
export interface CustomMockData {
  nodes: CustomNode[];
}

export const customMockData: CustomMockData = {
  nodes: [
    {
      id: 'node-1',
      name: 'pve-prod-01',
      status: 'online',
      cpu: { usage: 0.62, cores: 32 },
      memory: { used: 103079215104, total: 137438953472 }, // 96GB used of 128GB
      guests: [
        { 
          id: 101, 
          name: 'db-primary', 
          type: 'vm', 
          status: 'running', 
          cpu: 0.78, 
          memory: 34359738368, // 32GB
          disk: { used: 858993459200, total: 1099511627776 } // 800GB used of 1TB
        },
        { 
          id: 102, 
          name: 'web-prod-01', 
          type: 'vm', 
          status: 'running', 
          cpu: 0.32, 
          memory: 8589934592, // 8GB
          disk: { used: 32212254720, total: 107374182400 } // 30GB used of 100GB
        },
        { 
          id: 103, 
          name: 'web-prod-02', 
          type: 'vm', 
          status: 'running', 
          cpu: 0.27, 
          memory: 8589934592, // 8GB
          disk: { used: 34359738368, total: 107374182400 } // 32GB used of 100GB
        },
        { 
          id: 104, 
          name: 'redis-cache-01', 
          type: 'vm', 
          status: 'running', 
          cpu: 0.22, 
          memory: 16777216000, // 16GB
          disk: { used: 21474836480, total: 53687091200 } // 20GB used of 50GB
        },
        { 
          id: 201, 
          name: 'haproxy-01', 
          type: 'ct', 
          status: 'running', 
          cpu: 0.15, 
          memory: 2147483648, // 2GB
          disk: { used: 3221225472, total: 10737418240 } // 3GB used of 10GB
        },
        { 
          id: 202, 
          name: 'nginx-lb', 
          type: 'ct', 
          status: 'stopped', 
          cpu: 0, 
          memory: 2147483648, // 2GB
          disk: { used: 3221225472, total: 10737418240 } // 3GB used of 10GB
        },
        { 
          id: 105, 
          name: 'shared-db-cluster', 
          type: 'vm', 
          status: 'running', 
          cpu: 0.45, 
          memory: 16777216000, // 16GB
          disk: { used: 107374182400, total: 214748364800 }, // 100GB used of 200GB
          shared: true,
          primaryNode: 'node-1'
        },
        { 
          id: 106, 
          name: 'clustered-app', 
          type: 'vm', 
          status: 'running', 
          cpu: 0.35, 
          memory: 4294967296, // 4GB
          disk: { used: 21474836480, total: 53687091200 }, // 20GB used of 50GB
          shared: true,
          primaryNode: 'node-1'
        },
        { 
          id: 203, 
          name: 'clustered-service', 
          type: 'ct', 
          status: 'running', 
          cpu: 0.22, 
          memory: 2147483648, // 2GB
          disk: { used: 5368709120, total: 10737418240 }, // 5GB used of 10GB
          shared: true,
          primaryNode: 'node-1'
        },
        { 
          id: 107, 
          name: 'shared-storage', 
          type: 'vm', 
          status: 'stopped', 
          cpu: 0, 
          memory: 8589934592, // 8GB
          disk: { used: 536870912000, total: 1099511627776 }, // 500GB used of 1TB
          shared: true,
          primaryNode: 'node-2'
        }
      ]
    },
    {
      id: 'node-2',
      name: 'pve-prod-02',
      status: 'online',
      cpu: { usage: 0.58, cores: 32 },
      memory: { used: 90194313216, total: 137438953472 }, // 84GB used of 128GB
      guests: [
        { 
          id: 108, 
          name: 'db-replica-02', 
          type: 'vm', 
          status: 'running', 
          cpu: 0.42, 
          memory: 17179869184, // 16GB
          disk: { used: 504403158016, total: 1099511627776 } // 470GB used of 1TB
        },
        { 
          id: 109, 
          name: 'web-prod-03', 
          type: 'vm', 
          status: 'running', 
          cpu: 0.36, 
          memory: 8589934592, // 8GB
          disk: { used: 32212254720, total: 107374182400 } // 30GB used of 100GB
        },
        { 
          id: 110, 
          name: 'elasticsearch-01', 
          type: 'vm', 
          status: 'running', 
          cpu: 0.46, 
          memory: 17179869184, // 16GB
          disk: { used: 236223201280, total: 322122547200 } // 220GB used of 300GB
        },
        { 
          id: 111, 
          name: 'logstash-01', 
          type: 'vm', 
          status: 'running', 
          cpu: 0.32, 
          memory: 8589934592, // 8GB
          disk: { used: 42949672960, total: 107374182400 } // 40GB used of 100GB
        },
        { 
          id: 204, 
          name: 'nginx-01', 
          type: 'ct', 
          status: 'running', 
          cpu: 0.13, 
          memory: 2147483648, // 2GB
          disk: { used: 3221225472, total: 10737418240 } // 3GB used of 10GB
        },
        { 
          id: 107, 
          name: 'shared-storage', 
          type: 'vm', 
          status: 'running', 
          cpu: 0.55, 
          memory: 8589934592, // 8GB
          disk: { used: 536870912000, total: 1099511627776 }, // 500GB used of 1TB
          shared: true,
          primaryNode: 'node-2'
        },
        { 
          id: 112, 
          name: 'shared-backup', 
          type: 'vm', 
          status: 'running', 
          cpu: 0.18, 
          memory: 4294967296, // 4GB
          disk: { used: 214748364800, total: 322122547200 }, // 200GB used of 300GB
          shared: true,
          primaryNode: 'node-2'
        },
        { 
          id: 205, 
          name: 'shared-monitor', 
          type: 'ct', 
          status: 'running', 
          cpu: 0.12, 
          memory: 2147483648, // 2GB
          disk: { used: 21474836480, total: 53687091200 }, // 20GB used of 50GB
          shared: true,
          primaryNode: 'node-2'
        },
        { 
          id: 105, 
          name: 'shared-db-cluster', 
          type: 'vm', 
          status: 'stopped', 
          cpu: 0, 
          memory: 16777216000, // 16GB
          disk: { used: 107374182400, total: 214748364800 }, // 100GB used of 200GB
          shared: true,
          primaryNode: 'node-1'
        },
        { 
          id: 106, 
          name: 'clustered-app', 
          type: 'vm', 
          status: 'stopped', 
          cpu: 0, 
          memory: 4294967296, // 4GB
          disk: { used: 21474836480, total: 53687091200 }, // 20GB used of 50GB
          shared: true,
          primaryNode: 'node-1'
        }
      ]
    },
    {
      id: 'node-3',
      name: 'pve-dev-01',
      status: 'online',
      cpu: { usage: 0.38, cores: 16 },
      memory: { used: 24696061952, total: 34359738368 }, // 23GB used of 32GB
      guests: [
        { 
          id: 113, 
          name: 'dev-db-01', 
          type: 'vm', 
          status: 'running', 
          cpu: 0.25, 
          memory: 8589934592, // 8GB
          disk: { used: 107374182400, total: 214748364800 } // 100GB used of 200GB
        },
        { 
          id: 114, 
          name: 'dev-web-01', 
          type: 'vm', 
          status: 'running', 
          cpu: 0, 
          memory: 4294967296, // 4GB
          disk: { used: 21474836480, total: 53687091200 } // 20GB used of 50GB
        },
        { 
          id: 115, 
          name: 'dev-api-01', 
          type: 'vm', 
          status: 'running', 
          cpu: 0, 
          memory: 2147483648, // 2GB
          disk: { used: 10737418240, total: 32212254720 } // 10GB used of 30GB
        },
        { 
          id: 116, 
          name: 'dev-testing', 
          type: 'vm', 
          status: 'running', 
          cpu: 0, 
          memory: 1073741824, // 1GB
          disk: { used: 10737418240, total: 21474836480 } // 10GB used of 20GB
        },
        { 
          id: 117, 
          name: 'dev-jenkins', 
          type: 'vm', 
          status: 'running', 
          cpu: 0.05, 
          memory: 2147483648, // 2GB
          disk: { used: 32212254720, total: 53687091200 } // 30GB used of 50GB
        },
        { 
          id: 206, 
          name: 'dev-proxy', 
          type: 'ct', 
          status: 'running', 
          cpu: 0.02, 
          memory: 1073741824, // 1GB
          disk: { used: 5368709120, total: 10737418240 } // 5GB used of 10GB
        },
        { 
          id: 112, 
          name: 'shared-backup', 
          type: 'vm', 
          status: 'stopped', 
          cpu: 0, 
          memory: 4294967296, // 4GB
          disk: { used: 214748364800, total: 322122547200 }, // 200GB used of 300GB
          shared: true,
          primaryNode: 'node-2'
        },
        { 
          id: 205, 
          name: 'shared-monitor', 
          type: 'ct', 
          status: 'stopped', 
          cpu: 0, 
          memory: 2147483648, // 2GB
          disk: { used: 21474836480, total: 53687091200 }, // 20GB used of 50GB
          shared: true,
          primaryNode: 'node-2'
        },
        { 
          id: 203, 
          name: 'clustered-service', 
          type: 'ct', 
          status: 'stopped', 
          cpu: 0, 
          memory: 2147483648, // 2GB
          disk: { used: 5368709120, total: 10737418240 }, // 5GB used of 10GB
          shared: true,
          primaryNode: 'node-1'
        },
        { 
          id: 118, 
          name: 'dev-sandbox', 
          type: 'vm', 
          status: 'running', 
          cpu: 0.08, 
          memory: 4294967296, // 4GB
          disk: { used: 10737418240, total: 107374182400 } // 10GB used of 100GB
        }
      ]
    }
  ]
}; 