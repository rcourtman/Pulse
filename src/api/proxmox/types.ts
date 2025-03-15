import { ProxmoxNodeStatus, ProxmoxVM, ProxmoxContainer, ProxmoxEvent } from '../../types';

// Define method interfaces for the ProxmoxClient
export interface ProxmoxClientMethods {
  discoverNodeName(): Promise<string>;
  extractIpAddress(host: string): string;
  getNodeNameAsync(): Promise<string>;
  getNodeName(): string;
  getNodeStatus(): Promise<ProxmoxNodeStatus>;
  getVirtualMachines(): Promise<ProxmoxVM[]>;
  getContainers(): Promise<ProxmoxContainer[]>;
  getGuestResourceUsage(type: 'qemu' | 'lxc', vmid: number): Promise<any>;
  subscribeToEvents(callback: (event: ProxmoxEvent) => void): Promise<() => void>;
  determineEventType(event: any): 'node' | 'vm' | 'container' | 'storage' | 'pool';
  setupEventPolling(): void;
  isNodeInCluster(): Promise<{ isCluster: boolean; clusterName: string }>;
  getClusterResources(): Promise<{ vms: ProxmoxVM[]; containers: ProxmoxContainer[] }>;
} 