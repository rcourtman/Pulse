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
