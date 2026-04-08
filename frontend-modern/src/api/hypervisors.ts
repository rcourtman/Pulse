/**
 * Hypervisor management API client.
 *
 * Provides CRUD operations for configuring hypervisor/cloud provider connections.
 */

export interface HypervisorType {
  type: string;
  name: string;
  description: string;
}

export interface HypervisorInstance {
  id: string;
  name: string;
  type: string;
  host?: string;
  enabled: boolean;
  username?: string;
  region?: string;
  subscriptionId?: string;
  tenantId?: string;
  projectId?: string;
  datacenter?: string;
  verifySSL: boolean;
}

export interface HypervisorCreateRequest {
  name: string;
  type: string;
  host?: string;
  enabled: boolean;
  username?: string;
  password?: string;
  token?: string;
  verifySSL?: boolean;
  // Cloud-specific
  region?: string;
  accessKey?: string;
  secretKey?: string;
  subscriptionId?: string;
  tenantId?: string;
  clientId?: string;
  clientSecret?: string;
  projectId?: string;
  credentialsJson?: string;
  // VMware-specific
  datacenter?: string;
  // Libvirt-specific
  keyFile?: string;
}

export interface TestResult {
  id: string;
  type: string;
  success: boolean;
  message: string;
}

export class HypervisorAPI {
  /**
   * List all configured hypervisor instances.
   */
  static async list(): Promise<HypervisorInstance[]> {
    const response = await fetch('/api/hypervisors');
    if (!response.ok) throw new Error(`Failed to list hypervisors: ${response.statusText}`);
    return response.json();
  }

  /**
   * Add a new hypervisor instance.
   */
  static async add(instance: HypervisorCreateRequest): Promise<HypervisorInstance> {
    const response = await fetch('/api/hypervisors', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(instance),
    });
    if (!response.ok) throw new Error(`Failed to add hypervisor: ${response.statusText}`);
    return response.json();
  }

  /**
   * Update an existing hypervisor instance.
   */
  static async update(id: string, instance: Partial<HypervisorCreateRequest>): Promise<void> {
    const response = await fetch(`/api/hypervisors/${encodeURIComponent(id)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(instance),
    });
    if (!response.ok) throw new Error(`Failed to update hypervisor: ${response.statusText}`);
  }

  /**
   * Delete a hypervisor instance.
   */
  static async remove(id: string): Promise<void> {
    const response = await fetch(`/api/hypervisors/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
    if (!response.ok) throw new Error(`Failed to delete hypervisor: ${response.statusText}`);
  }

  /**
   * Test connectivity to a hypervisor instance.
   */
  static async test(id: string): Promise<TestResult> {
    const response = await fetch(`/api/hypervisors/${encodeURIComponent(id)}/test`, {
      method: 'POST',
    });
    if (!response.ok) throw new Error(`Failed to test hypervisor: ${response.statusText}`);
    return response.json();
  }

  /**
   * Get supported hypervisor types.
   */
  static async getTypes(): Promise<HypervisorType[]> {
    const response = await fetch('/api/hypervisors/types');
    if (!response.ok) throw new Error(`Failed to get hypervisor types: ${response.statusText}`);
    return response.json();
  }
}
