import type { NodeConfig } from '@/types/nodes';

export class NodesAPI {
  private static baseUrl = '/api/config/nodes';

  static async getNodes(): Promise<NodeConfig[]> {
    const response = await fetch(this.baseUrl);
    if (!response.ok) {
      throw new Error('Failed to fetch nodes');
    }
    // The API returns an array of nodes directly
    const nodes: NodeConfig[] = await response.json();
    return nodes;
  }

  static async addNode(node: NodeConfig): Promise<{ success: boolean; message?: string }> {
    const response = await fetch(this.baseUrl, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ node }),
    });
    
    if (!response.ok) {
      const error = await response.text();
      throw new Error(`Failed to add node: ${error}`);
    }
    
    return response.json();
  }

  static async updateNode(nodeId: string, node: NodeConfig): Promise<{ success: boolean; message?: string }> {
    const response = await fetch(`${this.baseUrl}/${nodeId}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ node }),
    });
    
    if (!response.ok) {
      const error = await response.text();
      throw new Error(`Failed to update node: ${error}`);
    }
    
    return response.json();
  }

  static async deleteNode(nodeId: string): Promise<{ success: boolean; message?: string }> {
    const response = await fetch(`${this.baseUrl}/${nodeId}`, {
      method: 'DELETE',
    });
    
    if (!response.ok) {
      const error = await response.text();
      throw new Error(`Failed to delete node: ${error}`);
    }
    
    return response.json();
  }

  static async testConnection(node: NodeConfig): Promise<{ 
    success: boolean; 
    message?: string;
    details?: {
      version?: string;
      nodes?: string[];
      datastores?: string[];
      latency?: number;
    };
  }> {
    // For new nodes (no ID), use the test-config endpoint
    // For existing nodes (with ID), use the node-specific test endpoint
    const endpoint = node.id ? `${this.baseUrl}/${node.id}/test` : `${this.baseUrl}/test-config`;
    const body = node.id ? {} : node; // Send full config for new nodes
    
    const response = await fetch(endpoint, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
    });
    
    if (!response.ok) {
      const error = await response.text();
      throw new Error(`Connection test failed: ${error}`);
    }
    
    const result = await response.json();
    
    // Convert backend response format to expected format
    if (result.status === 'success') {
      return {
        success: true,
        message: result.message,
        details: {
          latency: result.latency
        }
      };
    } else {
      throw new Error(result.message || 'Connection failed');
    }
  }
}