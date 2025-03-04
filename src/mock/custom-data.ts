/**
 * Custom Mock Data for Screenshots
 * 
 * This file contains custom mock data for generating screenshots.
 * It provides a consistent set of data with different guests for each node.
 */

export const customMockData = {
  nodes: [
    {
      id: 'node-1',
      name: 'pve-1',
      status: 'online',
      cpu: { usage: 0.45, cores: 8 },
      memory: { used: 8589934592, total: 17179869184 },
      guests: [
        { id: 'node1-vm1', name: 'node1-ubuntu-vm', type: 'vm', status: 'running', cpu: 0.32, memory: 2147483648 },
        { id: 'node1-vm2', name: 'node1-debian-vm', type: 'vm', status: 'stopped', cpu: 0, memory: 4294967296 },
        { id: 'node1-ct1', name: 'node1-alpine-ct', type: 'ct', status: 'running', cpu: 0.12, memory: 1073741824 },
        { id: 'node1-ct2', name: 'node1-nginx-ct', type: 'ct', status: 'paused', cpu: 0, memory: 536870912 }
      ]
    },
    {
      id: 'node-2',
      name: 'pve-2',
      status: 'online',
      cpu: { usage: 0.28, cores: 4 },
      memory: { used: 4294967296, total: 8589934592 },
      guests: [
        { id: 'node2-vm1', name: 'node2-debian-vm', type: 'vm', status: 'running', cpu: 0.18, memory: 3221225472 },
        { id: 'node2-vm2', name: 'node2-ubuntu-vm', type: 'vm', status: 'stopped', cpu: 0, memory: 2147483648 },
        { id: 'node2-ct1', name: 'node2-fedora-ct', type: 'ct', status: 'stopped', cpu: 0, memory: 536870912 },
        { id: 'node2-ct2', name: 'node2-redis-ct', type: 'ct', status: 'running', cpu: 0.22, memory: 1073741824 }
      ]
    },
    {
      id: 'node-3',
      name: 'pve-3',
      status: 'online',
      cpu: { usage: 0.65, cores: 16 },
      memory: { used: 12884901888, total: 34359738368 },
      guests: [
        { id: 'node3-vm1', name: 'node3-windows-vm', type: 'vm', status: 'running', cpu: 0.45, memory: 8589934592 },
        { id: 'node3-vm2', name: 'node3-centos-vm', type: 'vm', status: 'paused', cpu: 0, memory: 4294967296 },
        { id: 'node3-vm3', name: 'node3-fedora-vm', type: 'vm', status: 'stopped', cpu: 0, memory: 6442450944 },
        { id: 'node3-ct1', name: 'node3-ubuntu-ct', type: 'ct', status: 'running', cpu: 0.05, memory: 1073741824 },
        { id: 'node3-ct2', name: 'node3-postgres-ct', type: 'ct', status: 'running', cpu: 0.15, memory: 2147483648 }
      ]
    }
  ]
}; 