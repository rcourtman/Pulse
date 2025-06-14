const dnsResolver = require('../dnsResolver');
const dns = require('dns').promises;

// Mock the dns module
jest.mock('dns', () => ({
    promises: {
        resolve4: jest.fn(),
        resolve6: jest.fn()
    }
}));

// Mock the util.promisify
jest.mock('util', () => ({
    promisify: () => jest.fn()
}));

describe('DnsResolver', () => {
    beforeEach(() => {
        // Clear all mocks and caches
        jest.clearAllMocks();
        dnsResolver.clearCache();
    });

    describe('resolveHostname', () => {
        it('should resolve hostname to IP addresses', async () => {
            const mockIPs = ['192.168.1.10', '192.168.1.11', '192.168.1.12'];
            dns.resolve4.mockResolvedValue(mockIPs);
            dns.resolve6.mockResolvedValue([]);

            const result = await dnsResolver.resolveHostname('proxmox.lan');
            
            expect(result).toEqual(mockIPs);
            expect(dns.resolve4).toHaveBeenCalledWith('proxmox.lan');
        });

        it('should cache DNS results', async () => {
            const mockIPs = ['192.168.1.10'];
            dns.resolve4.mockResolvedValue(mockIPs);
            dns.resolve6.mockResolvedValue([]);

            // First call
            await dnsResolver.resolveHostname('test.lan');
            expect(dns.resolve4).toHaveBeenCalledTimes(1);

            // Second call should use cache
            await dnsResolver.resolveHostname('test.lan');
            expect(dns.resolve4).toHaveBeenCalledTimes(1); // Still only called once
        });

        it('should filter out failed IPs', async () => {
            const mockIPs = ['192.168.1.10', '192.168.1.11', '192.168.1.12'];
            dns.resolve4.mockResolvedValue(mockIPs);
            dns.resolve6.mockResolvedValue([]);

            // Mark one IP as failed
            dnsResolver.markHostFailed('192.168.1.11');

            const result = await dnsResolver.resolveHostname('proxmox.lan');
            
            expect(result).toEqual(['192.168.1.10', '192.168.1.12']);
            expect(result).not.toContain('192.168.1.11');
        });

        it('should handle DNS resolution failures gracefully', async () => {
            dns.resolve4.mockRejectedValue(new Error('DNS resolution failed'));
            dns.resolve6.mockRejectedValue(new Error('DNS resolution failed'));

            // Mock lookup to also fail
            const lookup = require('util').promisify();
            lookup.mockRejectedValue(new Error('Lookup failed'));

            await expect(dnsResolver.resolveHostname('invalid.lan'))
                .rejects.toThrow('No IP addresses found');
        });
    });

    describe('markHostFailed and isHostFailed', () => {
        it('should mark host as failed temporarily', async () => {
            const testIP = '192.168.1.10';
            
            expect(dnsResolver.isHostFailed(testIP)).toBe(false);
            
            dnsResolver.markHostFailed(testIP);
            expect(dnsResolver.isHostFailed(testIP)).toBe(true);
        });
    });

    describe('extractHostname', () => {
        it('should extract hostname from various URL formats', () => {
            const testCases = [
                { input: 'https://proxmox.lan:8006', expected: 'proxmox.lan' },
                { input: 'http://test.local:3000/path', expected: 'test.local' },
                { input: 'server.domain:8080', expected: 'server.domain' },
                { input: 'simple-hostname', expected: 'simple-hostname' }
            ];

            testCases.forEach(({ input, expected }) => {
                expect(dnsResolver.extractHostname(input)).toBe(expected);
            });
        });
    });

    describe('canResolve', () => {
        it('should return true for resolvable hostnames', async () => {
            dns.resolve4.mockResolvedValue(['192.168.1.10']);
            dns.resolve6.mockResolvedValue([]);

            const result = await dnsResolver.canResolve('valid.lan');
            expect(result).toBe(true);
        });

        it('should return false for unresolvable hostnames', async () => {
            dns.resolve4.mockRejectedValue(new Error('Not found'));
            dns.resolve6.mockRejectedValue(new Error('Not found'));
            
            const lookup = require('util').promisify();
            lookup.mockRejectedValue(new Error('Not found'));

            const result = await dnsResolver.canResolve('invalid.lan');
            expect(result).toBe(false);
        });
    });
});