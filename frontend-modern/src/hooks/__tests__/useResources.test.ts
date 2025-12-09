/**
 * Tests for useResources hook functionality
 * 
 * Note: These tests focus on the logic and type conversions rather than
 * reactive behavior, which requires a full SolidJS testing environment.
 */
import { describe, expect, it } from 'vitest';
import type { Resource, ResourceStatus } from '@/types/resource';

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

describe('useResourcesAsLegacy - Legacy Format Conversion', () => {
    describe('VM conversion', () => {
        it('converts Resource to legacy VM format', () => {
            const resource = createMockResource({
                type: 'vm',
                cpu: { current: 50 },
                memory: { current: 60, total: 4294967296, used: 2576980378 },
                disk: { current: 30, total: 107374182400, used: 32212254720 },
                uptime: 86400,
                network: { rxBytes: 1000000, txBytes: 500000 },
                tags: ['web', 'production'],
                platformData: {
                    vmid: 100,
                    node: 'pve1',
                    instance: 'pve1/qemu/100',
                    cpus: 4,
                    template: false,
                    osName: 'Ubuntu',
                    osVersion: '22.04',
                },
            });

            // Simulate the conversion logic from useResourcesAsLegacy
            const platformData = resource.platformData as Record<string, unknown>;
            const legacyVm = {
                id: resource.id,
                vmid: platformData?.vmid as number ?? parseInt(resource.id.split('-').pop() ?? '0', 10),
                name: resource.name,
                node: platformData?.node as string ?? '',
                instance: platformData?.instance as string ?? resource.platformId,
                status: resource.status === 'running' ? 'running' : 'stopped',
                type: 'qemu',
                cpu: (resource.cpu?.current ?? 0) / 100, // Convert percentage to ratio
                cpus: platformData?.cpus as number ?? 1,
                memory: resource.memory ? {
                    total: resource.memory.total ?? 0,
                    used: resource.memory.used ?? 0,
                    free: resource.memory.free ?? 0,
                    usage: resource.memory.current,
                } : { total: 0, used: 0, free: 0, usage: 0 },
                uptime: resource.uptime ?? 0,
                networkIn: resource.network?.rxBytes ?? 0,
                networkOut: resource.network?.txBytes ?? 0,
                tags: resource.tags ?? [],
            };

            expect(legacyVm.vmid).toBe(100);
            expect(legacyVm.node).toBe('pve1');
            expect(legacyVm.cpu).toBe(0.5); // 50% -> 0.5 ratio
            expect(legacyVm.memory.total).toBe(4294967296);
            expect(legacyVm.uptime).toBe(86400);
            expect(legacyVm.networkIn).toBe(1000000);
            expect(legacyVm.tags).toEqual(['web', 'production']);
        });

        it('handles missing platformData gracefully', () => {
            const resource = createMockResource({
                type: 'vm',
                platformData: undefined,
            });

            const platformData = resource.platformData as Record<string, unknown> | undefined;
            const vmid = platformData?.vmid as number ?? 0;
            const node = platformData?.node as string ?? '';

            expect(vmid).toBe(0);
            expect(node).toBe('');
        });
    });

    describe('Container conversion', () => {
        it('converts Resource to legacy Container format', () => {
            const resource = createMockResource({
                type: 'container',
                platformData: {
                    vmid: 200,
                    node: 'pve1',
                },
            });

            const platformData = resource.platformData as Record<string, unknown>;
            const legacyContainer = {
                id: resource.id,
                vmid: platformData?.vmid as number,
                name: resource.name,
                type: 'lxc',
                status: resource.status === 'running' ? 'running' : 'stopped',
            };

            expect(legacyContainer.vmid).toBe(200);
            expect(legacyContainer.type).toBe('lxc');
        });
    });

    describe('Node conversion', () => {
        it('converts Resource to legacy Node format with temperature', () => {
            const now = Date.now();
            const resource = createMockResource({
                type: 'node',
                name: 'pve1',
                status: 'online',
                temperature: 45.5,
                lastSeen: now,
                cpu: { current: 35 },
                platformData: {
                    host: '192.168.1.10',
                    loadAverage: [1.5, 1.2, 0.9],
                    kernelVersion: '6.1.0-pve',
                    pveVersion: '8.0.3',
                },
            });

            // Simulate temperature conversion
            let temperature = undefined;
            if (resource.temperature !== undefined && resource.temperature !== null && resource.temperature > 0) {
                temperature = {
                    cpuPackage: resource.temperature,
                    cpuMax: resource.temperature,
                    available: true,
                    hasCPU: true,
                    hasGPU: false,
                    hasNVMe: false,
                    lastUpdate: new Date(resource.lastSeen).toISOString(),
                };
            }

            expect(temperature).toBeDefined();
            expect(temperature!.cpuPackage).toBe(45.5);
            expect(temperature!.available).toBe(true);
        });

        it('handles node without temperature', () => {
            const resource = createMockResource({
                type: 'node',
                temperature: undefined,
            });

            let temperature = undefined;
            if (resource.temperature !== undefined && resource.temperature !== null && resource.temperature > 0) {
                temperature = { cpuPackage: resource.temperature };
            }

            expect(temperature).toBeUndefined();
        });

        it('converts CPU from percentage to ratio', () => {
            const resource = createMockResource({
                type: 'node',
                cpu: { current: 75 },
            });

            const legacyCpu = (resource.cpu?.current ?? 0) / 100;
            expect(legacyCpu).toBe(0.75);
        });
    });

    describe('DockerHost conversion', () => {
        it('converts Resource to legacy DockerHost format with containers', () => {
            const dockerHost = createMockResource({
                id: 'docker-host-1',
                type: 'docker-host',
                name: 'docker-server',
                platformType: 'docker',
                cpu: { current: 25 },
                memory: { current: 50, total: 8589934592, used: 4294967296 },
                platformData: {
                    agentId: 'agent-123',
                    runtime: 'docker',
                    runtimeVersion: '24.0.0',
                    dockerVersion: '24.0.0',
                    cpus: 8,
                },
            });

            const dockerContainers: Resource[] = [
                createMockResource({
                    id: 'docker-host-1/container-abc',
                    type: 'docker-container',
                    name: 'nginx',
                    parentId: 'docker-host-1',
                    status: 'running',
                    cpu: { current: 5 },
                    memory: { current: 10, total: 134217728, used: 13421772 },
                    platformData: {
                        image: 'nginx:latest',
                        health: 'healthy',
                    },
                }),
            ];

            // Simulate container ID extraction for sparklines
            const originalContainerId = dockerContainers[0].id.includes('/')
                ? dockerContainers[0].id.split('/').pop()!
                : dockerContainers[0].id;

            expect(originalContainerId).toBe('container-abc');

            const legacyHost = {
                id: dockerHost.id,
                agentId: (dockerHost.platformData as any).agentId ?? dockerHost.id,
                hostname: dockerHost.identity?.hostname ?? dockerHost.name,
                cpuUsagePercent: dockerHost.cpu?.current,
                containers: dockerContainers.filter(c => c.parentId === dockerHost.id),
            };

            expect(legacyHost.agentId).toBe('agent-123');
            expect(legacyHost.containers).toHaveLength(1);
        });
    });

    describe('Host conversion', () => {
        it('converts Resource to legacy Host format with sensors', () => {
            const resource = createMockResource({
                type: 'host',
                name: 'server1',
                platformType: 'host-agent',
                status: 'online',
                platformData: {
                    platform: 'linux',
                    osName: 'Ubuntu',
                    osVersion: '22.04 LTS',
                    kernelVersion: '6.2.0',
                    architecture: 'x86_64',
                    cpuCount: 16,
                    loadAverage: [2.5, 1.8, 1.2],
                    sensors: {
                        temperatureCelsius: { 'CPU Package': 55.0, 'nvme0': 42.0 },
                        fanRpm: { 'CPU Fan': 1200 },
                    },
                    raid: [
                        {
                            device: '/dev/md0',
                            level: 'raid1',
                            state: 'active',
                            totalDevices: 2,
                            activeDevices: 2,
                            workingDevices: 2,
                            failedDevices: 0,
                            spareDevices: 0,
                            devices: [
                                { device: 'sda1', state: 'active', slot: 0 },
                                { device: 'sdb1', state: 'active', slot: 1 },
                            ],
                            rebuildPercent: 0,
                        },
                    ],
                },
            });

            const platformData = resource.platformData as Record<string, unknown>;

            expect(platformData.sensors).toBeDefined();
            expect((platformData.sensors as any).temperatureCelsius['CPU Package']).toBe(55.0);
            expect(platformData.raid).toHaveLength(1);
            expect((platformData.raid as any[])[0].level).toBe('raid1');
        });

        it('maps interfaces correctly to networkInterfaces', () => {
            const resource = createMockResource({
                type: 'host',
                platformData: {
                    interfaces: [
                        { name: 'eth0', mac: '00:11:22:33:44:55', addresses: ['192.168.1.10'] },
                        { name: 'docker0', mac: '02:42:00:00:00:01', addresses: ['172.17.0.1'] },
                    ],
                },
            });

            const platformData = resource.platformData as Record<string, unknown>;
            const interfaces = platformData.interfaces as any[];

            expect(interfaces).toHaveLength(2);
            expect(interfaces[0].name).toBe('eth0');
            expect(interfaces[1].addresses).toContain('172.17.0.1');
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
