const axios = require('axios');
const semver = require('semver');
const fs = require('fs').promises;
const path = require('path');
const { exec } = require('child_process');
const { promisify } = require('util');
const execAsync = promisify(exec);

class UpdateManager {
    constructor() {
        this.githubRepo = 'rcourtman/Pulse';
        this.currentVersion = require('../package.json').version;
        this.updateInProgress = false;
    }

    /**
     * Check for available updates
     */
    async checkForUpdates() {
        try {
            console.log('[UpdateManager] Checking for updates...');
            
            // Fetch latest release from GitHub
            const response = await axios.get(
                `https://api.github.com/repos/${this.githubRepo}/releases/latest`,
                {
                    headers: {
                        'Accept': 'application/vnd.github.v3+json',
                        'User-Agent': 'Pulse-Update-Checker'
                    },
                    timeout: 10000
                }
            );

            const latestVersion = response.data.tag_name.replace('v', '');
            const updateAvailable = semver.gt(latestVersion, this.currentVersion);

            const updateInfo = {
                currentVersion: this.currentVersion,
                latestVersion,
                updateAvailable,
                isDocker: this.isDockerEnvironment(),
                releaseNotes: response.data.body || 'No release notes available',
                releaseUrl: response.data.html_url,
                publishedAt: response.data.published_at,
                assets: response.data.assets.map(asset => ({
                    name: asset.name,
                    size: asset.size,
                    downloadUrl: asset.browser_download_url
                }))
            };

            console.log(`[UpdateManager] Current version: ${this.currentVersion}, Latest version: ${latestVersion}, Docker: ${updateInfo.isDocker}`);
            return updateInfo;

        } catch (error) {
            console.error('[UpdateManager] Error checking for updates:', error.message);
            throw new Error(`Failed to check for updates: ${error.message}`);
        }
    }

    /**
     * Download update package
     */
    async downloadUpdate(downloadUrl, progressCallback) {
        try {
            console.log('[UpdateManager] Downloading update from:', downloadUrl);
            
            const tempDir = path.join(__dirname, '..', 'temp');
            await fs.mkdir(tempDir, { recursive: true });
            
            const tempFile = path.join(tempDir, 'update.tar.gz');
            
            // In test mode, create a mock tarball directly instead of downloading
            if (process.env.UPDATE_TEST_MODE === 'true' && downloadUrl.includes('/api/test/mock-update.tar.gz')) {
                console.log('[UpdateManager] Test mode: Creating mock update package...');
                
                const tar = require('tar');
                await tar.create({
                    gzip: true,
                    file: tempFile,
                    cwd: path.join(__dirname, '..'),
                    filter: (path) => {
                        return !path.includes('node_modules') && 
                               !path.includes('.git') && 
                               !path.includes('temp') &&
                               !path.includes('data/backups');
                    }
                }, ['.']);
                
                // Simulate download progress
                if (progressCallback) {
                    for (let i = 0; i <= 100; i += 10) {
                        progressCallback({ phase: 'download', progress: i });
                        await new Promise(resolve => setTimeout(resolve, 100));
                    }
                }
                
                console.log('[UpdateManager] Test mode: Mock package created successfully');
                return tempFile;
            }
            
            // Normal download for real updates
            const response = await axios({
                method: 'get',
                url: downloadUrl,
                responseType: 'stream',
                timeout: 300000 // 5 minutes
            });

            const totalSize = parseInt(response.headers['content-length'], 10);
            let downloadedSize = 0;

            const writer = require('fs').createWriteStream(tempFile);
            
            response.data.on('data', (chunk) => {
                downloadedSize += chunk.length;
                if (progressCallback) {
                    const progress = Math.round((downloadedSize / totalSize) * 100);
                    progressCallback({ phase: 'download', progress });
                }
            });

            response.data.pipe(writer);

            return new Promise((resolve, reject) => {
                writer.on('finish', () => resolve(tempFile));
                writer.on('error', (error) => {
                    console.error('[UpdateManager] Writer error:', error);
                    reject(error);
                });
                response.data.on('error', (error) => {
                    console.error('[UpdateManager] Response stream error:', error);
                    writer.destroy();
                    reject(error);
                });
            });

        } catch (error) {
            console.error('[UpdateManager] Error downloading update:', error);
            console.error('[UpdateManager] Error details:', {
                message: error.message,
                code: error.code,
                response: error.response?.status,
                responseData: error.response?.data
            });
            throw new Error(`Failed to download update: ${error.message || error.toString()}`);
        }
    }

    /**
     * Check if running in Docker
     */
    isDockerEnvironment() {
        return process.env.DOCKER_DEPLOYMENT === 'true' || 
               require('fs').existsSync('/.dockerenv') ||
               (process.env.container === 'docker');
    }

    /**
     * Apply update
     */
    async applyUpdate(updateFile, progressCallback) {
        if (this.updateInProgress) {
            throw new Error('Update already in progress');
        }

        // Check if running in Docker
        if (this.isDockerEnvironment()) {
            throw new Error(
                'Automatic updates are not supported in Docker deployments. ' +
                'Please update your Docker image by pulling the latest version:\n' +
                'docker pull rcourtman/pulse:latest\n' +
                'or update your docker-compose.yml to use the new version tag.'
            );
        }

        this.updateInProgress = true;

        try {
            console.log('[UpdateManager] Applying update...');
            
            // Create backup directory
            const backupDir = path.join(__dirname, '..', 'backup', `backup-${Date.now()}`);
            await fs.mkdir(backupDir, { recursive: true });

            if (progressCallback) {
                progressCallback({ phase: 'backup', progress: 0 });
            }

            // Backup critical files
            const filesToBackup = [
                '.env',
                'data/metrics.db',
                'data/acknowledgements.json'
            ];

            for (let i = 0; i < filesToBackup.length; i++) {
                const file = filesToBackup[i];
                const sourcePath = path.join(__dirname, '..', file);
                const backupPath = path.join(backupDir, file);
                
                try {
                    await fs.mkdir(path.dirname(backupPath), { recursive: true });
                    await fs.copyFile(sourcePath, backupPath);
                } catch (error) {
                    if (error.code !== 'ENOENT') {
                        console.warn(`[UpdateManager] Warning: Could not backup ${file}:`, error.message);
                    }
                }

                if (progressCallback) {
                    const progress = Math.round(((i + 1) / filesToBackup.length) * 100);
                    progressCallback({ phase: 'backup', progress });
                }
            }

            if (progressCallback) {
                progressCallback({ phase: 'extract', progress: 0 });
            }

            // Extract update
            const tempExtractDir = path.join(__dirname, '..', 'temp', 'extract');
            await fs.mkdir(tempExtractDir, { recursive: true });

            await execAsync(`tar -xzf ${updateFile} -C ${tempExtractDir}`);

            if (progressCallback) {
                progressCallback({ phase: 'extract', progress: 100 });
            }

            const pulseDir = path.join(__dirname, '..');

            if (progressCallback) {
                progressCallback({ phase: 'apply', progress: 50 });
            }

            // Apply update files
            console.log('[UpdateManager] Extracting update files...');
            
            // List files to update (exclude config and data)
            const updateFiles = await fs.readdir(tempExtractDir);
            for (const file of updateFiles) {
                if (file === '.env' || file === 'data') continue;
                
                const sourcePath = path.join(tempExtractDir, file);
                const destPath = path.join(pulseDir, file);
                
                // Remove existing file/directory
                try {
                    await fs.rm(destPath, { recursive: true, force: true });
                } catch (e) {
                    // Ignore errors
                }
                
                // Copy new file/directory
                await execAsync(`cp -rf "${sourcePath}" "${destPath}"`);
            }

            // Install dependencies
            console.log('[UpdateManager] Installing dependencies...');
            await execAsync(`cd "${pulseDir}" && rm -rf node_modules && npm install --omit=dev`);

            if (progressCallback) {
                progressCallback({ phase: 'apply', progress: 100 });
            }

            // Schedule restart
            console.log('[UpdateManager] Scheduling restart...');
            setTimeout(async () => {
                console.log('[UpdateManager] Attempting restart...');
                
                // In test mode or development, just exit (user needs to restart manually)
                if (process.env.UPDATE_TEST_MODE === 'true' || process.env.NODE_ENV === 'development') {
                    console.log('[UpdateManager] Test/Dev mode: Please restart the server manually');
                    console.log('[UpdateManager] Exiting in 3 seconds...');
                    setTimeout(() => {
                        process.exit(0);
                    }, 3000);
                    return;
                }
                
                // For production deployments, try various restart methods
                try {
                    // Try systemctl first (Linux with systemd)
                    await execAsync('sudo systemctl restart pulse');
                    console.log('[UpdateManager] Restarted via systemctl');
                } catch (error) {
                    try {
                        // Try pm2 restart
                        await execAsync('pm2 restart pulse');
                        console.log('[UpdateManager] Restarted via pm2');
                    } catch (error) {
                        // Last resort: exit and hope something restarts us
                        console.log('[UpdateManager] No restart mechanism found, exiting...');
                        process.exit(0);
                    }
                }
            }, 2000);

            // Cleanup
            await fs.rm(path.join(__dirname, '..', 'temp'), { recursive: true, force: true });

            return {
                success: true,
                message: 'Update applied successfully. The application will restart automatically.'
            };

        } catch (error) {
            console.error('[UpdateManager] Error applying update:', error.message);
            this.updateInProgress = false;
            throw new Error(`Failed to apply update: ${error.message}`);
        }
    }

    /**
     * Get update status
     */
    getUpdateStatus() {
        return {
            updateInProgress: this.updateInProgress,
            currentVersion: this.currentVersion
        };
    }
}

module.exports = UpdateManager;