const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

// --- Configuration ---
const BASE_URL = process.env.PULSE_URL || 'http://localhost:7655'; // Allow overriding via env var
const OUTPUT_DIR = path.resolve(__dirname, '../docs/images');
const DESKTOP_VIEWPORT = { width: 1440, height: 900 }; // Standard viewport
const WAIT_OPTIONS = { waitUntil: 'networkidle', timeout: 15000 }; // Increased timeout, networkidle
const OVERLAY_SELECTOR = '#loading-overlay';

// Define the sections to capture - ONLY ESSENTIAL SCREENSHOTS
const sections = [
    // Dashboard: Main view
    { name: '01-dashboard', 
      action: async (page) => {
          console.log('  Action: Waiting for dashboard content to load...');
          // Wait for the first row that DOES NOT contain the loading text TD
          await page.locator('#main-table tbody tr:not(:has(td:text("Loading data...")))').first().waitFor({ state: 'visible', timeout: 30000 });
          console.log('  Action: Dashboard content loaded.');
      }
    },

    // Storage View
    { name: '02-storage-view',
      action: async (page) => {
        console.log('  Action: Clicking Storage tab');
        await page.locator('[data-tab="storage"]').click();
        console.log('  Action: Waiting for storage container to be visible');
        await page.locator('#storage').waitFor({ state: 'visible', timeout: 10000 });
        
        // Wait for storage data to load
        try {
          await page.locator('#storage .storage-table tbody tr').first().waitFor({ state: 'visible', timeout: 15000 });
          console.log('  Action: Storage data loaded');
        } catch (e) {
          console.log('  Warning: Storage data may not be fully loaded');
        }
        
        await page.waitForTimeout(500);
      }
    },

    // Unified Backups View
    { name: '03-unified-backups-view',
      action: async (page) => {
        console.log('  Action: Clicking Backups tab');
        await page.locator('[data-tab="unified"]').click();
        console.log('  Action: Waiting for unified backups container to be visible');
        await page.locator('#unified').waitFor({ state: 'visible', timeout: 10000 });
        
        // Wait for backups data to load
        try {
          await page.locator('#unified-backups-tbody tr').first().waitFor({ state: 'visible', timeout: 5000 });
          console.log('  Action: Backups data loaded');
        } catch (e) {
          console.log('  Warning: No backup data available, capturing empty view');
        }
        
        await page.waitForTimeout(1000);
      }
    },

    // Thresholds View
    { name: '04-thresholds-view',
      action: async (page) => {
        console.log('  Action: Ensuring Main tab is active');
        // Ensure main tab is active
        const mainTabIsActive = await page.locator('[data-tab="main"].active').isVisible();
        if (!mainTabIsActive) {
             await page.locator('[data-tab="main"]').click();
             await page.waitForLoadState('networkidle', { timeout: 5000 });
        }

        console.log('  Action: Clicking Thresholds toggle to activate thresholds mode');
        // Click the thresholds toggle checkbox label to activate thresholds mode
        const thresholdsToggleLabel = page.locator('label:has(#toggle-thresholds-checkbox)');
        await thresholdsToggleLabel.waitFor({ state: 'visible', timeout: 10000 });
        await thresholdsToggleLabel.click();
        
        console.log('  Action: Waiting for thresholds mode changes');
        // Give it time for the UI to update
        await page.waitForTimeout(1000);
        
        console.log('  Action: Thresholds view is now active');
        
        // Brief wait to ensure everything is rendered
        await page.waitForTimeout(800);
      },
      postAction: async (page) => {
        console.log('  Action: Deactivating thresholds mode');
        // Toggle thresholds off again
        const thresholdsToggleLabel = page.locator('label:has(#toggle-thresholds-checkbox)');
        await thresholdsToggleLabel.click();
        await page.waitForTimeout(300);
      }
    },
    
    // Line Graph/Charts View
    { name: '05-charts-view', 
      action: async (page) => {
        console.log('  Action: Ensuring Main tab is active');
        // Ensure main tab is active
        const mainTabIsActive = await page.locator('[data-tab="main"].active').isVisible();
        if (!mainTabIsActive) {
             await page.locator('[data-tab="main"]').click();
             await page.waitForLoadState('networkidle', { timeout: 5000 });
        }

        // Hide node summary cards for cleaner charts view
        console.log('  Action: Hiding node summary cards');
        await page.locator('#node-summary-cards-container').evaluate(element => element.style.display = 'none');

        // Filter to show only LXC containers for cleaner view
        console.log('  Action: Clicking LXC filter');
        const lxcFilterLabel = page.locator('label[for="filter-lxc"]');
        await lxcFilterLabel.waitFor({ state: 'visible', timeout: 10000 });
        await lxcFilterLabel.click();
        await page.waitForTimeout(300);

        console.log('  Action: Clicking charts toggle label');
        const chartsToggleCheckbox = page.locator('#toggle-charts-checkbox');
        const isChecked = await chartsToggleCheckbox.isChecked();
        if (!isChecked) {
            const chartsToggleLabel = page.locator('label:has(#toggle-charts-checkbox)');
            await chartsToggleLabel.waitFor({ state: 'visible', timeout: 10000 });
            await chartsToggleLabel.click();
        }
        
        console.log('  Action: Waiting for charts to appear');
        await page.waitForFunction(() => {
            const mainContainer = document.getElementById('main');
            return mainContainer && mainContainer.classList.contains('charts-mode');
        }, { timeout: 5000 });
        
        // Brief wait to ensure charts are fully rendered
        await page.waitForTimeout(800);
        console.log('  Action: Charts are now visible');
        
        // Hover over a chart to show tooltip
        console.log('  Action: Hovering over a chart to show tooltip');
        try {
            const firstChart = page.locator('[id^="chart-"][id$="-cpu"] svg').first();
            await firstChart.waitFor({ state: 'visible', timeout: 5000 });
            
            const box = await firstChart.boundingBox();
            if (box) {
                const hoverX = box.x + (box.width * 0.8);
                const hoverY = box.y + (box.height * 0.5);
                
                await page.mouse.move(hoverX, hoverY);
                await page.waitForTimeout(300);
                console.log('  Action: Tooltip should now be visible');
            }
        } catch (e) {
            console.log('  Warning: Could not hover over chart for tooltip');
        }
      },
      postAction: async (page) => {
        console.log('  Action: Resetting view');
        // Toggle charts off again
        const chartsToggleCheckbox = page.locator('#toggle-charts-checkbox');
        const isChecked = await chartsToggleCheckbox.isChecked();
        if (isChecked) {
            const chartsToggleLabel = page.locator('label:has(#toggle-charts-checkbox)');
            await chartsToggleLabel.click();
            await page.waitForTimeout(300);
        }
        
        // Reset filter to show all
        const allFilterLabel = page.locator('label[for="filter-all"]');
        await allFilterLabel.click();
        await page.waitForTimeout(300);
        
        // Show node summary cards again
        await page.locator('#node-summary-cards-container').evaluate(element => element.style.display = '');
      }
    },

    // Alerts/Thresholds View
    { name: '06-alerts-view',
      action: async (page) => {
        console.log('  Action: Ensuring Main tab is active');
        // Ensure main tab is active
        const mainTabIsActive = await page.locator('[data-tab="main"].active').isVisible();
        if (!mainTabIsActive) {
            await page.locator('[data-tab="main"]').click();
            await page.waitForLoadState('networkidle', { timeout: 5000 });
        }

        console.log('  Action: Clicking Alerts toggle to activate alerts mode');
        // Click the alerts toggle checkbox label to activate alerts mode
        const alertsToggleCheckbox = page.locator('#toggle-alerts-checkbox');
        const isChecked = await alertsToggleCheckbox.isChecked();
        if (!isChecked) {
            const alertsToggleLabel = page.locator('label:has(#toggle-alerts-checkbox)');
            await alertsToggleLabel.waitFor({ state: 'visible', timeout: 10000 });
            await alertsToggleLabel.click();
        }
        
        console.log('  Action: Waiting for alerts mode changes');
        // Give it time for the UI to update
        await page.waitForTimeout(1000);
        
        console.log('  Action: Alerts view is now active');
        
        // Brief wait to ensure everything is rendered
        await page.waitForTimeout(800);
      },
      postAction: async (page) => {
        console.log('  Action: Deactivating alerts mode');
        // Toggle alerts off again
        const alertsToggleCheckbox = page.locator('#toggle-alerts-checkbox');
        const isChecked = await alertsToggleCheckbox.isChecked();
        if (isChecked) {
            const alertsToggleLabel = page.locator('label:has(#toggle-alerts-checkbox)');
            await alertsToggleLabel.click();
            await page.waitForTimeout(300);
        }
      }
    }
];

async function captureScreenshotsForViewport(browser, sectionsToCapture, viewport, viewportName) {
    console.log(`\n--- Starting ${viewportName} captures ---`);
    
    const context = await browser.newContext({
        viewport: viewport,
        ignoreHTTPSErrors: true,
        deviceScaleFactor: 2 // 2x for retina quality
    });
    const page = await context.newPage();
    
    // Get CDP session for better screenshot quality
    const client = await page.context().newCDPSession(page);
    

    console.log(`Navigating to base URL...`);
    await page.goto(BASE_URL, WAIT_OPTIONS);

    // Wait for fonts to load
    console.log('Waiting for fonts to load...');
    await page.evaluate(() => document.fonts.ready);
    
    // Wait for the loading overlay to disappear
    console.log(`Waiting for overlay to disappear...`);
    await page.locator(OVERLAY_SELECTOR).waitFor({ state: 'hidden', timeout: 20000 });
    console.log('Overlay hidden.');
    
    // Additional wait for rendering to complete
    await page.waitForTimeout(2000);
    
    // Force a repaint to ensure crisp rendering
    await page.evaluate(() => {
        document.body.style.display = 'none';
        document.body.offsetHeight; // Force reflow
        document.body.style.display = '';
    });
    
    await page.waitForTimeout(500);

    // Ensure Dark Mode
    console.log('Ensuring dark mode is active...');
    const isDarkMode = await page.evaluate(() => document.documentElement.classList.contains('dark'));
    if (!isDarkMode) {
        console.log(' Dark mode not active, forcing dark mode...');
        await page.evaluate(() => {
            document.documentElement.classList.add('dark');
            localStorage.setItem('theme', 'dark');
        });
        await page.waitForTimeout(300);
        console.log(' Dark mode activated.');
    } else {
        console.log(' Dark mode already active.');
    }

    console.log(`Starting section captures.`);

    for (const section of sectionsToCapture) {
        const screenshotPath = path.join(OUTPUT_DIR, `${section.name}.png`);
        console.log(`\nCapturing section: ${section.name}...`);

        try {
            // Perform specific actions if needed
            if (section.action) {
                console.log(`  Performing action for ${section.name}...`);
                await section.action(page);
                await page.waitForLoadState('networkidle', { timeout: 10000 });
                console.log('  Action completed and network idle.');
            }

            // Wait a bit more for any final rendering
            await page.waitForTimeout(500);
            
            // Take the screenshot using CDP with WebP for better quality
            const webpPath = screenshotPath.replace('.png', '.webp');
            console.log(`  Saving screenshot to: ${webpPath}`);
            
            try {
                // Use CDP with WebP format for higher quality
                const { data } = await client.send('Page.captureScreenshot', {
                    format: 'webp',
                    quality: 100,
                    fromSurface: true, // Capture from surface for better quality
                    captureBeyondViewport: false
                });
                
                fs.writeFileSync(webpPath, Buffer.from(data, 'base64'));
                
            } catch (cdpError) {
                console.log('  CDP screenshot failed, falling back to standard method');
                // Fallback to standard screenshot with WebP
                await page.screenshot({ 
                    path: webpPath, 
                    fullPage: false,
                    omitBackground: false,
                    animations: 'disabled',
                    type: 'jpeg',
                    quality: 100
                });
            }

            console.log(`  Successfully captured ${section.name}`);

            // Perform post-action if defined
            if (section.postAction) {
                console.log(`  Performing post-action for ${section.name}...`);
                await section.postAction(page);
                await page.waitForLoadState('networkidle', { timeout: 5000 });
                console.log('  Post-action completed.');
            }

        } catch (error) {
            console.error(`  Failed to capture section ${section.name}: ${error.message}`);
        }
    }

    await context.close();
    console.log(`\n${viewportName} captures completed.`);
}

async function takeScreenshots() {
    console.log(`Starting screenshot capture for ${BASE_URL}...`);
    console.log(`Outputting to: ${OUTPUT_DIR}`);

    // Clean up existing WebP files
    if (fs.existsSync(OUTPUT_DIR)) {
        console.log(`\nCleaning up existing *.webp files in ${OUTPUT_DIR}...`);
        const files = fs.readdirSync(OUTPUT_DIR);
        let deletedCount = 0;
        files.forEach(file => {
            const ext = path.extname(file).toLowerCase();
            if (ext === '.webp') {
                const filePath = path.join(OUTPUT_DIR, file);
                try {
                    fs.unlinkSync(filePath);
                    deletedCount++;
                } catch (err) {
                    console.error(`  Error deleting file ${file}: ${err.message}`);
                }
            }
        });
        console.log(`Cleanup finished. Deleted ${deletedCount} WebP file(s).`);
    } else {
        console.log('Output directory does not exist, no cleanup needed.');
    }

    // Ensure output directory exists
    if (!fs.existsSync(OUTPUT_DIR)) {
        console.log(`Creating directory: ${OUTPUT_DIR}`);
        fs.mkdirSync(OUTPUT_DIR, { recursive: true });
    }

    let browser;
    try {
        browser = await chromium.launch({
            headless: true,
            args: [
                '--no-sandbox',
                '--disable-setuid-sandbox',
                '--disable-dev-shm-usage'
            ]
        });

        // Capture desktop screenshots only (no mobile)
        await captureScreenshotsForViewport(browser, sections, DESKTOP_VIEWPORT, 'desktop');

    } catch (error) {
        console.error(`Error during screenshot process: ${error}`);
        process.exitCode = 1;
    } finally {
        if (browser) {
            await browser.close();
            console.log('\nBrowser closed.');
        }
    }

    console.log('\nScreenshot capture finished.');
}

takeScreenshots();