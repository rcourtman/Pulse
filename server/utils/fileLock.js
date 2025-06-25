const fs = require('fs').promises;
const path = require('path');
const crypto = require('crypto');

class FileLock {
    constructor() {
        this.locks = new Map();
        this.lockDir = path.join(__dirname, '../../data/.locks');
        this.maxWaitTime = 5000; // 5 seconds max wait
        this.retryInterval = 50; // Check every 50ms
        this.lockTimeout = 10000; // Locks expire after 10 seconds
        
        // Ensure lock directory exists
        this.ensureLockDir();
    }
    
    async ensureLockDir() {
        try {
            await fs.mkdir(this.lockDir, { recursive: true });
        } catch (error) {
            console.error('[FileLock] Failed to create lock directory:', error);
        }
    }
    
    getLockPath(filePath) {
        const hash = crypto.createHash('md5').update(filePath).digest('hex');
        return path.join(this.lockDir, `${hash}.lock`);
    }
    
    async acquireLock(filePath, timeout = this.maxWaitTime) {
        const lockPath = this.getLockPath(filePath);
        const lockId = crypto.randomBytes(16).toString('hex');
        const startTime = Date.now();
        
        while (Date.now() - startTime < timeout) {
            try {
                // Try to create lock file exclusively
                const lockData = {
                    pid: process.pid,
                    lockId,
                    timestamp: Date.now(),
                    file: filePath
                };
                
                await fs.writeFile(lockPath, JSON.stringify(lockData), { flag: 'wx' });
                
                // Store lock info
                this.locks.set(filePath, {
                    lockId,
                    lockPath,
                    acquired: Date.now()
                });
                
                return lockId;
            } catch (error) {
                if (error.code === 'EEXIST') {
                    // Lock exists, check if it's expired
                    try {
                        const lockContent = await fs.readFile(lockPath, 'utf-8');
                        const lockData = JSON.parse(lockContent);
                        
                        if (Date.now() - lockData.timestamp > this.lockTimeout) {
                            // Lock is stale, remove it
                            await fs.unlink(lockPath);
                            continue;
                        }
                    } catch (parseError) {
                        // Invalid lock file, remove it
                        await fs.unlink(lockPath).catch(() => {});
                        continue;
                    }
                    
                    // Wait before retrying
                    await new Promise(resolve => setTimeout(resolve, this.retryInterval));
                } else {
                    throw error;
                }
            }
        }
        
        throw new Error(`Failed to acquire lock for ${filePath} after ${timeout}ms`);
    }
    
    async releaseLock(filePath, lockId) {
        const lockInfo = this.locks.get(filePath);
        
        if (!lockInfo || lockInfo.lockId !== lockId) {
            throw new Error('Invalid lock ID or lock not held');
        }
        
        try {
            await fs.unlink(lockInfo.lockPath);
            this.locks.delete(filePath);
        } catch (error) {
            if (error.code !== 'ENOENT') {
                console.error('[FileLock] Error releasing lock:', error);
            }
        }
    }
    
    async withLock(filePath, operation) {
        let lockId;
        try {
            lockId = await this.acquireLock(filePath);
            return await operation();
        } finally {
            if (lockId) {
                await this.releaseLock(filePath, lockId);
            }
        }
    }
    
    // Clean up stale locks on startup
    async cleanupStaleLocks() {
        try {
            const files = await fs.readdir(this.lockDir);
            let cleaned = 0;
            
            for (const file of files) {
                if (!file.endsWith('.lock')) continue;
                
                const lockPath = path.join(this.lockDir, file);
                try {
                    const content = await fs.readFile(lockPath, 'utf-8');
                    const lockData = JSON.parse(content);
                    
                    if (Date.now() - lockData.timestamp > this.lockTimeout) {
                        await fs.unlink(lockPath);
                        cleaned++;
                    }
                } catch (error) {
                    // Invalid lock file, remove it
                    await fs.unlink(lockPath).catch(() => {});
                    cleaned++;
                }
            }
            
            if (cleaned > 0) {
                console.log(`[FileLock] Cleaned up ${cleaned} stale locks`);
            }
        } catch (error) {
            console.error('[FileLock] Error cleaning up stale locks:', error);
        }
    }
}

module.exports = new FileLock();