import { NodeConfig } from '../types/nodes';

export class NodesAPI {
  private static readonly baseUrl = '/api/config/nodes';

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
      body: JSON.stringify(node),
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
      body: JSON.stringify(node),
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
    status: string;
    message?: string; 
    isCluster?: boolean;
    nodeCount?: number;
    clusterNodeCount?: number;
    datastoreCount?: number;
  }> {
    const response = await fetch(`${this.baseUrl}/test-connection`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(node),
    });
    
    if (!response.ok) {
      const error = await response.text();
      throw new Error(error);
    }
    
    return response.json();
  }
}