/**
 * Tests for useResources hook functionality
 * 
 * Note: These tests focus on the logic and type conversions rather than
 * reactive behavior, which requires a full SolidJS testing environment.
 */
import { describe, expect, it } from 'vitest';
import { createRoot } from 'solid-js';
import type { Resource, ResourceStatus } from '@/types/resource';
import { useAIChatResources, useAlertsResources } from '@/hooks/useResources';

// Helper to create mock resources for testing conversion logic
function createMockResource(overrides: Partial<Resource> = {}): Resource {
    return {
        id: 'test-resource-1',
        type: 'vm',
        name: 'test-vm',
        displayName: 'Test VM',
        platformId: 'pve1',
        platformType: 'proxmox-pve',
        sourceType: 'api',
        status: 'running',
        lastSeen: Date.now(),
        cpu: { current: 25.5 },
        memory: { current: 50, total: 4294967296, used: 2147483648 },
        disk: { current: 30, total: 107374182400, used: 32212254720 },
        ...overrides,
    };
}

function createMockLegacyStore(stateOverrides: Record<string, unknown> = {}) {
    return {
        state: {
            resources: [],
            nodes: [],
            vms: [],
            containers: [],
            storage: [],
            hosts: [],
            dockerHosts: [],
            pbs: [],
            pmg: [],
            ...stateOverrides,
        },
    };
}

describe('useResources - Resource Filtering Logic', () => {
    describe('byType filtering', () => {
        const resources: Resource[] = [
            createMockResource({ id: '1', type: 'vm' }),
            createMockResource({ id: '2', type: 'vm' }),
            createMockResource({ id: '3', type: 'container' }),
            createMockResource({ id: '4', type: 'node' }),
            createMockResource({ id: '5', type: 'docker-container' }),
        ];

        it('filters resources by single type', () => {
            const vms = resources.filter(r => r.type === 'vm');
            expect(vms).toHaveLength(2);
        });

        it('returns empty array when no matches', () => {
            const filtered = resources.filter(r => r.type === 'pbs');
            expect(filtered).toHaveLength(0);
        });
    });

    describe('byPlatform filtering', () => {
        const resources: Resource[] = [
            createMockResource({ id: '1', platformType: 'proxmox-pve' }),
            createMockResource({ id: '2', platformType: 'proxmox-pve' }),
            createMockResource({ id: '3', platformType: 'docker' }),
            createMockResource({ id: '4', platformType: 'host-agent' }),
        ];

        it('filters resources by platform', () => {
            const pveResources = resources.filter(r => r.platformType === 'proxmox-pve');
            expect(pveResources).toHaveLength(2);
        });

        it('filters Docker resources', () => {
            const dockerResources = resources.filter(r => r.platformType === 'docker');
            expect(dockerResources).toHaveLength(1);
        });
    });

    describe('children filtering', () => {
        const resources: Resource[] = [
            createMockResource({ id: 'node-1', type: 'node' }),
            createMockResource({ id: 'vm-1', type: 'vm', parentId: 'node-1' }),
            createMockResource({ id: 'vm-2', type: 'vm', parentId: 'node-1' }),
            createMockResource({ id: 'vm-3', type: 'vm', parentId: 'node-2' }),
        ];

        it('finds children of a parent', () => {
            const children = resources.filter(r => r.parentId === 'node-1');
            expect(children).toHaveLength(2);
        });

        it('returns empty array for parent with no children', () => {
            const children = resources.filter(r => r.parentId === 'node-3');
            expect(children).toHaveLength(0);
        });
    });

    describe('complex filtering', () => {
        const resources: Resource[] = [
            createMockResource({ id: '1', type: 'vm', status: 'running', platformType: 'proxmox-pve' }),
            createMockResource({ id: '2', type: 'vm', status: 'stopped', platformType: 'proxmox-pve' }),
            createMockResource({ id: '3', type: 'container', status: 'running', platformType: 'proxmox-pve' }),
            createMockResource({ id: '4', type: 'docker-container', status: 'running', platformType: 'docker' }),
        ];

        it('filters by multiple criteria', () => {
            const runningPveVms = resources.filter(r =>
                r.type === 'vm' &&
                r.status === 'running' &&
                r.platformType === 'proxmox-pve'
            );
            expect(runningPveVms).toHaveLength(1);
        });

        it('filters by search term in name', () => {
            const searchTerm = 'test';
            const matched = resources.filter(r =>
                r.name.toLowerCase().includes(searchTerm.toLowerCase())
            );
            expect(matched).toHaveLength(4);
        });
    });

    describe('status counts', () => {
        const resources: Resource[] = [
            createMockResource({ id: '1', status: 'running' }),
            createMockResource({ id: '2', status: 'running' }),
            createMockResource({ id: '3', status: 'stopped' }),
            createMockResource({ id: '4', status: 'online' }),
            createMockResource({ id: '5', status: 'degraded' }),
        ];

        it('counts resources by status', () => {
            const counts: Record<ResourceStatus, number> = {
                online: 0,
                offline: 0,
                running: 0,
                stopped: 0,
                degraded: 0,
                paused: 0,
                unknown: 0,
            };

            for (const r of resources) {
                if (r.status in counts) {
                    counts[r.status]++;
                }
            }

            expect(counts.running).toBe(2);
            expect(counts.stopped).toBe(1);
            expect(counts.online).toBe(1);
            expect(counts.degraded).toBe(1);
        });
    });

    describe('topByCpu sorting', () => {
        const resources: Resource[] = [
            createMockResource({ id: '1', name: 'low', cpu: { current: 10 } }),
            createMockResource({ id: '2', name: 'high', cpu: { current: 90 } }),
            createMockResource({ id: '3', name: 'medium', cpu: { current: 50 } }),
            createMockResource({ id: '4', name: 'no-cpu', cpu: undefined }), // Explicitly no CPU data
        ];

        it('sorts by CPU descending and limits results', () => {
            const sorted = [...resources]
                .filter(r => r.cpu && r.cpu.current > 0)
                .sort((a, b) => (b.cpu?.current ?? 0) - (a.cpu?.current ?? 0))
                .slice(0, 2);

            expect(sorted).toHaveLength(2);
            expect(sorted[0].name).toBe('high');
            expect(sorted[1].name).toBe('medium');
        });

        it('excludes resources without CPU data', () => {
            const withCpu = resources.filter(r => r.cpu && r.cpu.current > 0);
            expect(withCpu).toHaveLength(3);
        });
    });
});

describe('Fallback Logic', () => {
    describe('hasUnifiedResources check', () => {
        it('returns true when resources array has items', () => {
            const resources: Resource[] = [createMockResource()];
            expect(resources.length > 0).toBe(true);
        });

        it('returns false when resources array is empty', () => {
            const resources: Resource[] = [];
            expect(resources.length > 0).toBe(false);
        });
    });

    describe('legacy array fallback', () => {
        // This tests the concept - actual implementation uses WebSocket store
        it('uses legacy array when unified resources not available', () => {
            const unifiedResources: Resource[] = [];
            const legacyVms = [{ id: 'vm-1', name: 'Legacy VM' }];

            const hasUnifiedResources = unifiedResources.length > 0;
            const result = hasUnifiedResources ? unifiedResources : legacyVms;

            expect(result).toEqual(legacyVms);
        });

        it('uses unified resources when available', () => {
            const unifiedResources: Resource[] = [createMockResource({ id: 'unified-1' })];
            const legacyVms = [{ id: 'vm-1', name: 'Legacy VM' }];

            const hasUnifiedResources = unifiedResources.length > 0;
            const result = hasUnifiedResources ? unifiedResources : legacyVms;

            expect(result).toEqual(unifiedResources);
        });
    });
});

describe('useAlertsResources', () => {
    it('exposes all resource types needed by alerts consumers', () => {
        const store = createMockLegacyStore({
            nodes: [{ id: 'node-1' }],
            vms: [{ id: 'vm-1' }],
            containers: [{ id: 'ct-1' }],
            storage: [{ id: 'storage-1' }],
            hosts: [{ id: 'host-1' }],
            dockerHosts: [{ id: 'docker-host-1' }],
        });

        let dispose = () => {};
        const selectors = createRoot((d) => {
            dispose = d;
            return useAlertsResources(store as any);
        });

        expect(typeof selectors.nodes).toBe('function');
        expect(typeof selectors.vms).toBe('function');
        expect(typeof selectors.containers).toBe('function');
        expect(typeof selectors.storage).toBe('function');
        expect(typeof selectors.hosts).toBe('function');
        expect(typeof selectors.dockerHosts).toBe('function');
        expect(typeof selectors.ready).toBe('function');
        expect(selectors.nodes()).toHaveLength(1);
        expect(selectors.vms()).toHaveLength(1);
        expect(selectors.containers()).toHaveLength(1);
        expect(selectors.storage()).toHaveLength(1);
        expect(selectors.hosts()).toHaveLength(1);
        expect(selectors.dockerHosts()).toHaveLength(1);

        dispose();
    });

    it('ready signal is true when nodes are available', () => {
        const store = createMockLegacyStore({
            nodes: [{ id: 'node-1' }],
        });

        let dispose = () => {};
        const selectors = createRoot((d) => {
            dispose = d;
            return useAlertsResources(store as any);
        });

        expect(selectors.ready()).toBe(true);

        dispose();
    });
});

describe('useAIChatResources', () => {
    it('exposes all resource types needed by AI chat consumers', () => {
        const store = createMockLegacyStore({
            nodes: [{ id: 'node-1' }],
            vms: [{ id: 'vm-1' }],
            containers: [{ id: 'ct-1' }],
            dockerHosts: [{ id: 'docker-host-1' }],
            hosts: [{ id: 'host-1' }],
        });

        let dispose = () => {};
        const selectors = createRoot((d) => {
            dispose = d;
            return useAIChatResources(store as any);
        });

        expect(typeof selectors.nodes).toBe('function');
        expect(typeof selectors.vms).toBe('function');
        expect(typeof selectors.containers).toBe('function');
        expect(typeof selectors.dockerHosts).toBe('function');
        expect(typeof selectors.hosts).toBe('function');
        expect(typeof selectors.isCluster).toBe('function');
        expect(selectors.nodes()).toHaveLength(1);
        expect(selectors.vms()).toHaveLength(1);
        expect(selectors.containers()).toHaveLength(1);
        expect(selectors.dockerHosts()).toHaveLength(1);
        expect(selectors.hosts()).toHaveLength(1);

        dispose();
    });

    it('isCluster is true when multiple nodes exist', () => {
        const store = createMockLegacyStore({
            nodes: [{ id: 'node-1' }, { id: 'node-2' }],
        });

        let dispose = () => {};
        const selectors = createRoot((d) => {
            dispose = d;
            return useAIChatResources(store as any);
        });

        expect(selectors.isCluster()).toBe(true);

        dispose();
    });
});

describe('Stale legacy-only consumer detection', () => {
    it('unified selectors return unified conversion output, not raw legacy arrays', () => {
        const legacyNodes = [{ id: 'legacy-node-1', name: 'Legacy Node' }];
        const legacyVms = [{ id: 'legacy-vm-1', name: 'Legacy VM', cpu: 0.01 }];

        const store = createMockLegacyStore({
            resources: [
                createMockResource({
                    id: 'node-unified-1',
                    type: 'node',
                    name: 'Unified Node',
                    platformData: { host: 'pve1.local' },
                }),
                createMockResource({
                    id: 'vm-unified-101',
                    type: 'vm',
                    name: 'Unified VM',
                    cpu: { current: 55 },
                    platformData: {
                        vmid: 101,
                        node: 'pve1',
                        instance: 'pve1/qemu/101',
                    },
                }),
            ],
            nodes: legacyNodes,
            vms: legacyVms,
        });

        let dispose = () => {};
        const selectors = createRoot((d) => {
            dispose = d;
            return useAlertsResources(store as any);
        });

        expect(selectors.nodes()).not.toEqual(legacyNodes);
        expect(selectors.nodes()[0].id).toBe('node-unified-1');
        expect(selectors.vms()).not.toEqual(legacyVms);
        expect(selectors.vms()[0].id).toBe('vm-unified-101');
        expect(selectors.vms()[0].cpu).toBe(0.55);

        dispose();
    });

    it('useAlertsResources storage prefers unified conversion when unified resources are populated', () => {
        const legacyStorage = [{
            id: 'legacy-storage-1',
            name: 'backup-ds',
            node: 'legacy-pbs',
            instance: 'pbs-1',
            type: 'pbs',
            status: 'offline',
            used: 90,
        }];

        const store = createMockLegacyStore({
            resources: [
                createMockResource({
                    id: 'datastore-unified-1',
                    type: 'datastore',
                    name: 'backup-ds',
                    status: 'running',
                    disk: { current: 40, total: 100, used: 40, free: 60 },
                    platformData: {
                        pbsInstanceId: 'pbs-1',
                        pbsInstanceName: 'legacy-pbs',
                        type: 'pbs',
                    },
                }),
            ],
            storage: legacyStorage,
        });

        let dispose = () => {};
        const selectors = createRoot((d) => {
            dispose = d;
            return useAlertsResources(store as any);
        });

        expect(selectors.storage()).toHaveLength(1);
        expect(selectors.storage()[0].id).toBe('datastore-unified-1');
        expect(selectors.storage()[0].status).toBe('available');
        expect(selectors.storage()[0].used).toBe(40);

        dispose();
    });

    it('useAlertsResources ready signal is false when no nodes available', () => {
        const store = createMockLegacyStore({
            resources: [createMockResource({ id: 'vm-only-1', type: 'vm' })],
            nodes: [],
        });

        let dispose = () => {};
        const selectors = createRoot((d) => {
            dispose = d;
            return useAlertsResources(store as any);
        });

        expect(selectors.ready()).toBe(false);

        dispose();
    });

    it('useAIChatResources selectors prefer unified conversion when unified resources are populated', () => {
        const store = createMockLegacyStore({
            resources: [
                createMockResource({
                    id: 'node-unified-1',
                    type: 'node',
                    name: 'Unified Node',
                    platformData: { host: 'pve1.local' },
                }),
                createMockResource({
                    id: 'vm-unified-101',
                    type: 'vm',
                    name: 'Unified VM',
                    cpu: { current: 75 },
                    platformData: {
                        vmid: 101,
                        node: 'pve1',
                        instance: 'pve1/qemu/101',
                    },
                }),
                createMockResource({
                    id: 'ct-unified-201',
                    type: 'container',
                    name: 'Unified CT',
                    platformData: {
                        vmid: 201,
                        node: 'pve1',
                        instance: 'pve1/lxc/201',
                    },
                }),
                createMockResource({
                    id: 'docker-host-unified-1',
                    type: 'docker-host',
                    name: 'docker-host-1',
                    status: 'online',
                    platformData: { agentId: 'agent-1', runtime: 'docker' },
                }),
                createMockResource({
                    id: 'docker/container-unified-1',
                    type: 'docker-container',
                    name: 'nginx',
                    parentId: 'docker-host-unified-1',
                    status: 'running',
                    platformData: { image: 'nginx:latest' },
                }),
                createMockResource({
                    id: 'host-unified-1',
                    type: 'host',
                    name: 'host-1',
                    status: 'online',
                }),
            ],
            nodes: [{ id: 'legacy-node-1' }],
            vms: [{ id: 'legacy-vm-1', cpu: 0.01 }],
            containers: [{ id: 'legacy-ct-1' }],
            dockerHosts: [{ id: 'legacy-docker-host-1' }],
            hosts: [{ id: 'legacy-host-1' }],
        });

        let dispose = () => {};
        const selectors = createRoot((d) => {
            dispose = d;
            return useAIChatResources(store as any);
        });

        expect(selectors.nodes()).toHaveLength(1);
        expect(selectors.nodes()[0].id).toBe('node-unified-1');
        expect(selectors.vms()).toHaveLength(1);
        expect(selectors.vms()[0].id).toBe('vm-unified-101');
        expect(selectors.vms()[0].cpu).toBe(0.75);
        expect(selectors.containers()).toHaveLength(1);
        expect(selectors.containers()[0].id).toBe('ct-unified-201');
        expect(selectors.dockerHosts()).toHaveLength(1);
        expect(selectors.dockerHosts()[0].id).toBe('docker-host-unified-1');
        expect(selectors.dockerHosts()[0].containers).toHaveLength(1);
        expect(selectors.dockerHosts()[0].containers[0].id).toBe('container-unified-1');
        expect(selectors.hosts()).toHaveLength(1);
        expect(selectors.hosts()[0].id).toBe('host-unified-1');

        dispose();
    });

    it('useAIChatResources isCluster is false for single node', () => {
        const store = createMockLegacyStore({
            resources: [
                createMockResource({
                    id: 'node-unified-1',
                    type: 'node',
                    name: 'Unified Node',
                    platformData: { host: 'single.local' },
                }),
            ],
            nodes: [{ id: 'legacy-node-1' }, { id: 'legacy-node-2' }],
        });

        let dispose = () => {};
        const selectors = createRoot((d) => {
            dispose = d;
            return useAIChatResources(store as any);
        });

        expect(selectors.nodes()).toHaveLength(1);
        expect(selectors.nodes()[0].id).toBe('node-unified-1');
        expect(selectors.isCluster()).toBe(false);

        dispose();
    });
});
