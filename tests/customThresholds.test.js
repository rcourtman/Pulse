// Mock fs module before requiring the threshold manager
jest.mock('fs', () => ({
    promises: {
        mkdir: jest.fn(),
        readFile: jest.fn(),
        writeFile: jest.fn()
    }
}));

const fs = require('fs').promises;
const path = require('path');
const thresholdManagerInstance = require('../customThresholds');

// Mock console to avoid test output clutter
jest.spyOn(console, 'log').mockImplementation(() => {});
jest.spyOn(console, 'error').mockImplementation(() => {});

describe('Custom Threshold Manager', () => {
    let thresholdManager;
    let mockConfigPath;

    beforeEach(() => {
        thresholdManager = thresholdManagerInstance;
        mockConfigPath = thresholdManager.configPath;
        
        // Reset all mocks
        jest.clearAllMocks();
        
        // Clear cache for clean state
        thresholdManager.cache.clear();
    });

    afterEach(() => {
        // Clean up cache
        if (thresholdManager) {
            thresholdManager.cache.clear();
        }
    });

    describe('Initialization', () => {
        test('should initialize successfully with existing config file', async () => {
            const mockThresholds = {
                'endpoint1:100': {
                    endpointId: 'endpoint1',
                    vmid: '100',
                    thresholds: {
                        cpu: { warning: 70, critical: 90 },
                        memory: { warning: 80, critical: 95 }
                    },
                    enabled: true,
                    createdAt: new Date().toISOString()
                }
            };

            fs.mkdir.mockResolvedValue();
            fs.readFile.mockResolvedValue(JSON.stringify(mockThresholds));

            await thresholdManager.init();

            expect(thresholdManager.initialized).toBe(true);
            expect(thresholdManager.cache.size).toBe(1);
        });

        test('should create new config file when none exists', async () => {
            const enoentError = new Error('File not found');
            enoentError.code = 'ENOENT';

            fs.mkdir.mockResolvedValue();
            fs.readFile.mockRejectedValue(enoentError);
            fs.writeFile.mockResolvedValue();

            await thresholdManager.init();

            expect(thresholdManager.initialized).toBe(true);
            expect(fs.writeFile).toHaveBeenCalled();
        });
    });

    describe('Key Generation', () => {
        test('should generate correct cache key format', () => {
            const key = thresholdManager.generateKey('pve-main', 'node1', '100');
            expect(key).toBe('pve-main:100');
        });

        test('should handle special characters in endpoint and vmid', () => {
            const key = thresholdManager.generateKey('pve-test.local', 'node-1', 'ct-200');
            expect(key).toBe('pve-test.local:ct-200');
        });

        test('should be consistent regardless of node parameter', () => {
            const key1 = thresholdManager.generateKey('pve1', 'node1', '100');
            const key2 = thresholdManager.generateKey('pve1', 'node2', '100');
            expect(key1).toBe(key2); // Node migration support
        });
    });

    describe('Getting Thresholds', () => {
        beforeEach(async () => {
            fs.writeFile.mockResolvedValue();
            
            // Set up cache with test data using the real API
            await thresholdManager.setThresholds('pve1', 'node1', '100', {
                cpu: { warning: 75, critical: 90 },
                memory: { warning: 85, critical: 95 }
            });
            await thresholdManager.setThresholds('pve1', 'node1', '200', {
                cpu: { warning: 60, critical: 80 },
                disk: { warning: 90, critical: 98 }
            });
        });

        test('should return custom thresholds when configured', () => {
            const thresholds = thresholdManager.getThresholds('pve1', 'node1', '100');
            
            expect(thresholds).not.toBeNull();
            expect(thresholds.thresholds.cpu.warning).toBe(75);
            expect(thresholds.thresholds.cpu.critical).toBe(90);
            expect(thresholds.thresholds.memory.warning).toBe(85);
        });

        test('should return null when no custom thresholds exist', () => {
            const thresholds = thresholdManager.getThresholds('pve1', 'node1', '999');
            expect(thresholds).toBeNull();
        });

        test('should return null for different endpoint', () => {
            const thresholds = thresholdManager.getThresholds('pve2', 'node1', '100');
            expect(thresholds).toBeNull();
        });

        test('should work regardless of node name due to migration support', () => {
            const thresholds1 = thresholdManager.getThresholds('pve1', 'node1', '100');
            const thresholds2 = thresholdManager.getThresholds('pve1', 'node2', '100');
            
            expect(thresholds1).toEqual(thresholds2);
        });
    });

    describe('Setting Thresholds', () => {
        beforeEach(() => {
            fs.writeFile.mockResolvedValue();
        });

        test('should set valid threshold configuration', async () => {
            const validThresholds = {
                cpu: { warning: 70, critical: 85 },
                memory: { warning: 80, critical: 90 }
            };

            const result = await thresholdManager.setThresholds('pve1', 'node1', '300', validThresholds);

            expect(result).toBe(true);
            const stored = thresholdManager.getThresholds('pve1', 'node1', '300');
            expect(stored).not.toBeNull();
            expect(stored.thresholds.cpu.warning).toBe(70);
            expect(stored.createdAt).toBeDefined();
            expect(fs.writeFile).toHaveBeenCalled();
        });

        test('should validate threshold values', async () => {
            const invalidThresholds = {
                cpu: { warning: 95, critical: 85 } // Warning higher than critical
            };

            await expect(
                thresholdManager.setThresholds('pve1', 'node1', '400', invalidThresholds)
            ).rejects.toThrow(/critical threshold must be greater than warning threshold/);
        });

        test('should reject thresholds outside valid range', async () => {
            const outOfRangeThresholds = {
                cpu: { warning: 150, critical: 200 } // Over 100%
            };

            await expect(
                thresholdManager.setThresholds('pve1', 'node1', '500', outOfRangeThresholds)
            ).rejects.toThrow();
        });

        test('should handle partial thresholds gracefully', async () => {
            const partialThresholds = {
                cpu: { warning: 70, critical: 85 }
                // memory and disk thresholds missing
            };

            const result = await thresholdManager.setThresholds('pve1', 'node1', '700', partialThresholds);
            expect(result).toBe(true);

            const stored = thresholdManager.getThresholds('pve1', 'node1', '700');
            expect(stored.thresholds.cpu).toBeDefined();
            expect(stored.thresholds.memory).toBeUndefined();
        });

        test('should update existing thresholds', async () => {
            // Set initial thresholds
            const initial = {
                cpu: { warning: 70, critical: 85 }
            };
            await thresholdManager.setThresholds('pve1', 'node1', '800', initial);

            // Update with new values
            const updated = {
                cpu: { warning: 75, critical: 90 },
                memory: { warning: 80, critical: 95 }
            };
            await thresholdManager.setThresholds('pve1', 'node1', '800', updated);

            const stored = thresholdManager.getThresholds('pve1', 'node1', '800');
            expect(stored.thresholds.cpu.warning).toBe(75);
            expect(stored.thresholds.memory.warning).toBe(80);
            expect(fs.writeFile).toHaveBeenCalledTimes(2);
        });
    });

    describe('Removing Thresholds', () => {
        beforeEach(async () => {
            fs.writeFile.mockResolvedValue();
            
            // Set up some test thresholds
            await thresholdManager.setThresholds('pve1', 'node1', '100', {
                cpu: { warning: 70, critical: 85 }
            });
            await thresholdManager.setThresholds('pve1', 'node1', '200', {
                memory: { warning: 80, critical: 90 }
            });
        });

        test('should remove existing threshold configuration', async () => {
            expect(thresholdManager.getThresholds('pve1', 'node1', '100')).not.toBeNull();

            const result = await thresholdManager.removeThresholds('pve1', 'node1', '100');

            expect(result).toBe(true);
            expect(thresholdManager.getThresholds('pve1', 'node1', '100')).toBeNull();
            expect(fs.writeFile).toHaveBeenCalled();
        });

        test('should handle removal of non-existent thresholds gracefully', async () => {
            const result = await thresholdManager.removeThresholds('pve1', 'node1', '999');
            expect(result).toBe(false);
        });

        test('should not affect other threshold configurations', async () => {
            await thresholdManager.removeThresholds('pve1', 'node1', '100');

            expect(thresholdManager.getThresholds('pve1', 'node1', '200')).not.toBeNull();
        });
    });

    describe('File Operations', () => {
        test('should handle file save errors gracefully', async () => {
            const saveError = new Error('Disk full');
            fs.writeFile.mockRejectedValue(saveError);

            await expect(
                thresholdManager.setThresholds('pve1', 'node1', '100', {
                    cpu: { warning: 70, critical: 85 }
                })
            ).rejects.toThrow('Disk full');
        });

        test('should create data directory if it does not exist', async () => {
            fs.mkdir.mockResolvedValue();
            fs.readFile.mockResolvedValue('{}');

            await thresholdManager.loadThresholds();

            expect(fs.mkdir).toHaveBeenCalledWith(
                path.dirname(mockConfigPath),
                { recursive: true }
            );
        });

        test('should save thresholds in correct JSON format', async () => {
            fs.writeFile.mockResolvedValue();
            
            await thresholdManager.setThresholds('pve1', 'node1', '100', {
                cpu: { warning: 70, critical: 85 }
            });

            const saveCall = fs.writeFile.mock.calls[0];
            expect(saveCall[0]).toBe(mockConfigPath);
            expect(saveCall[2]).toBe('utf8');
            
            const savedData = JSON.parse(saveCall[1]);
            expect(savedData).toHaveProperty('pve1:100');
        });
    });

    describe('Edge Cases and Error Handling', () => {
        test('should handle empty threshold configuration', async () => {
            fs.writeFile.mockResolvedValue();
            
            const emptyThresholds = {};

            const result = await thresholdManager.setThresholds('pve1', 'node1', '100', emptyThresholds);
            expect(result).toBe(true);

            const stored = thresholdManager.getThresholds('pve1', 'node1', '100');
            expect(stored.createdAt).toBeDefined();
        });

        test('should handle very large cache sizes', async () => {
            fs.writeFile.mockResolvedValue();
            
            // Add many threshold configurations
            for (let i = 0; i < 100; i++) {
                await thresholdManager.setThresholds('pve1', 'node1', String(i), {
                    cpu: { warning: 70, critical: 85 }
                });
            }

            expect(thresholdManager.cache.size).toBe(100);
            expect(thresholdManager.getThresholds('pve1', 'node1', '50')).not.toBeNull();
        });
    });

    describe('Bulk Operations', () => {
        test('should get all threshold configurations', async () => {
            fs.writeFile.mockResolvedValue();
            
            await thresholdManager.setThresholds('pve1', 'node1', '100', {
                cpu: { warning: 70, critical: 85 }
            });
            await thresholdManager.setThresholds('pve2', 'node1', '200', {
                memory: { warning: 80, critical: 90 }
            });

            const allConfigs = thresholdManager.getAllThresholds();

            expect(Array.isArray(allConfigs)).toBe(true);
            expect(allConfigs.length).toBe(2);
            expect(allConfigs.some(config => config.vmid === '100')).toBe(true);
            expect(allConfigs.some(config => config.vmid === '200')).toBe(true);
        });

        test('should get thresholds by endpoint', async () => {
            fs.writeFile.mockResolvedValue();
            
            await thresholdManager.setThresholds('pve1', 'node1', '100', {
                cpu: { warning: 70, critical: 85 }
            });
            await thresholdManager.setThresholds('pve1', 'node1', '200', {
                memory: { warning: 80, critical: 90 }
            });
            await thresholdManager.setThresholds('pve2', 'node1', '300', {
                cpu: { warning: 60, critical: 75 }
            });

            const pve1Configs = thresholdManager.getThresholdsByEndpoint('pve1');
            const pve2Configs = thresholdManager.getThresholdsByEndpoint('pve2');

            expect(pve1Configs.length).toBe(2);
            expect(pve2Configs.length).toBe(1);
            expect(pve1Configs.every(config => config.endpointId === 'pve1')).toBe(true);
            expect(pve2Configs.every(config => config.endpointId === 'pve2')).toBe(true);
        });

        test('should export threshold configurations', () => {
            // Add some test data directly to cache
            thresholdManager.cache.set('pve1:100', {
                endpointId: 'pve1',
                vmid: '100',
                thresholds: { cpu: { warning: 70, critical: 85 } },
                createdAt: '2024-01-01T00:00:00.000Z'
            });

            const exported = thresholdManager.exportThresholds();

            expect(exported).toHaveProperty('exportedAt');
            expect(exported).toHaveProperty('version');
            expect(exported.version).toBe('1.0');
            expect(exported.thresholds).toHaveLength(1);
            expect(exported.thresholds[0].vmid).toBe('100');
        });

        test('should get threshold statistics', async () => {
            fs.writeFile.mockResolvedValue();
            
            // Add multiple configurations
            await thresholdManager.setThresholds('pve1', 'node1', '100', {
                cpu: { warning: 70, critical: 85 }
            });
            await thresholdManager.setThresholds('pve1', 'node1', '200', {
                memory: { warning: 80, critical: 90 }
            });

            const stats = thresholdManager.getStatistics();

            expect(stats).toHaveProperty('total');
            expect(stats).toHaveProperty('byEndpoint');
            expect(stats.total).toBe(2);
        });
    });

    describe('Threshold Management', () => {
        test('should toggle threshold configurations', async () => {
            fs.writeFile.mockResolvedValue();
            
            // Set up a threshold configuration
            await thresholdManager.setThresholds('pve1', 'node1', '100', {
                cpu: { warning: 70, critical: 85 }
            });

            // Disable it
            const result = await thresholdManager.toggleThresholds('pve1', 'node1', '100', false);
            expect(result).toBe(true);

            const config = thresholdManager.getThresholds('pve1', 'node1', '100');
            expect(config.enabled).toBe(false);
            expect(config.updatedAt).toBeDefined();

            // Re-enable it
            await thresholdManager.toggleThresholds('pve1', 'node1', '100', true);
            const updatedConfig = thresholdManager.getThresholds('pve1', 'node1', '100');
            expect(updatedConfig.enabled).toBe(true);
        });

        test('should handle toggle for non-existent configuration', async () => {
            await expect(
                thresholdManager.toggleThresholds('pve1', 'node1', '999', true)
            ).rejects.toThrow('Threshold configuration not found');
        });

        test('should validate threshold values correctly', () => {
            // Test CPU thresholds
            const validCpuThresholds = {
                cpu: { warning: 70, critical: 85 }
            };
            const validated = thresholdManager.validateThresholds(validCpuThresholds);
            expect(validated.cpu.warning).toBe(70);
            expect(validated.cpu.critical).toBe(85);

            // Test invalid CPU thresholds (warning >= critical)
            const invalidCpuThresholds = {
                cpu: { warning: 90, critical: 85 }
            };
            expect(() => {
                thresholdManager.validateThresholds(invalidCpuThresholds);
            }).toThrow('CPU critical threshold must be greater than warning threshold');
        });

        test('should validate memory thresholds correctly', () => {
            const validMemoryThresholds = {
                memory: { warning: 80, critical: 95 }
            };
            const validated = thresholdManager.validateThresholds(validMemoryThresholds);
            expect(validated.memory.warning).toBe(80);
            expect(validated.memory.critical).toBe(95);

            // Test invalid memory thresholds
            const invalidMemoryThresholds = {
                memory: { warning: 95, critical: 80 }
            };
            expect(() => {
                thresholdManager.validateThresholds(invalidMemoryThresholds);
            }).toThrow('Memory critical threshold must be greater than warning threshold');
        });

        test('should validate disk thresholds correctly', () => {
            const validDiskThresholds = {
                disk: { warning: 85, critical: 95 }
            };
            const validated = thresholdManager.validateThresholds(validDiskThresholds);
            expect(validated.disk.warning).toBe(85);
            expect(validated.disk.critical).toBe(95);

            // Test invalid disk thresholds
            const invalidDiskThresholds = {
                disk: { warning: 98, critical: 90 }
            };
            expect(() => {
                thresholdManager.validateThresholds(invalidDiskThresholds);
            }).toThrow('Disk critical threshold must be greater than warning threshold');
        });
    });

    describe('Integration with Alert System', () => {
        test('should store threshold configurations with proper structure', async () => {
            fs.writeFile.mockResolvedValue();
            
            await thresholdManager.setThresholds('pve1', 'node1', '100', {
                cpu: { warning: 75, critical: 90 },
                memory: { warning: 80, critical: 95 }
            });

            const config = thresholdManager.getThresholds('pve1', 'node1', '100');
            
            // Verify structure for alert system integration
            expect(config).toHaveProperty('endpointId', 'pve1');
            expect(config).toHaveProperty('vmid', '100');
            expect(config).toHaveProperty('thresholds');
            expect(config).toHaveProperty('enabled', true);
            expect(config).toHaveProperty('createdAt');
            expect(config).toHaveProperty('updatedAt');

            // Verify threshold values are accessible
            expect(config.thresholds.cpu.warning).toBe(75);
            expect(config.thresholds.cpu.critical).toBe(90);
            expect(config.thresholds.memory.warning).toBe(80);
            expect(config.thresholds.memory.critical).toBe(95);
        });

        test('should handle partial threshold configurations', async () => {
            fs.writeFile.mockResolvedValue();
            
            // Set only CPU thresholds
            await thresholdManager.setThresholds('pve1', 'node1', '200', {
                cpu: { warning: 70, critical: 85 }
            });

            const config = thresholdManager.getThresholds('pve1', 'node1', '200');
            
            expect(config.thresholds.cpu).toBeDefined();
            expect(config.thresholds.memory).toBeUndefined();
            expect(config.thresholds.disk).toBeUndefined();
        });
    });
});