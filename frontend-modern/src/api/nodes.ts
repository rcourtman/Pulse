import type { NodeConfig, NodesResponse } from '@/types/nodes';

export class NodesAPI {
  private static baseUrl = '/api/config/nodes';

  static async getNodes(): Promise<NodeConfig[]> {
    const response = await fetch(this.baseUrl);
    if (!response.ok) {
      throw new Error('Failed to fetch nodes');
    }
    const data: NodesResponse = await response.json();
    
    // Transform to flat array with type field
    const nodes: NodeConfig[] = [];
    
    data.pve_instances?.forEach(node => {
      nodes.push({ ...node, type: 'pve' as const });
    });
    
    data.pbs_instances?.forEach(node => {
      nodes.push({ ...node, type: 'pbs' as const });
    });
    
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
    };
  }> {
    const response = await fetch(`${this.baseUrl}/test`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ node }),
    });
    
    if (!response.ok) {
      const error = await response.text();
      throw new Error(`Connection test failed: ${error}`);
    }
    
    return response.json();
  }
}