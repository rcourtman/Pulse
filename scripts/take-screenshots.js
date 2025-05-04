const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

// --- Configuration ---
const BASE_URL = process.env.PULSE_URL || 'http://localhost:7655'; // Allow overriding via env var
const OUTPUT_DIR = path.resolve(__dirname, '../docs/images');
const VIEWPORT = { width: 1440, height: 900 }; // Match common M1 Air scaled resolution (16:10)
const WAIT_OPTIONS = { waitUntil: 'networkidle', timeout: 15000 }; // Increased timeout, networkidle
const OVERLAY_SELECTOR = '#loading-overlay';

// Define the sections to capture
// Placeholder selectors/navigation logic will need refinement
const sections = [
    // Dashboard: Wait for guest row, capture FULL page
    { name: '01-dashboard', 
      fullPage: true, 
      action: async (page) => {
          console.log('  Action: Waiting for dashboard content to load (checking for removal of loading text)...');
          // Wait for the first row that DOES NOT contain the loading text TD
          await page.locator('#main-table tbody tr:not(:has(td:text("Loading data...")))').first().waitFor({ state: 'visible', timeout: 30000 });
          console.log('  Action: Dashboard content loaded (loading text gone).');
      }
    },

    // Node View: Click tab, wait for table rows, capture nodes tab content
    { name: '02-node-view',
      screenshotTarget: '#nodes',
      action: async (page) => {
        console.log('  Action: Clicking Nodes tab');
        await page.locator('[data-tab="nodes"]').click();
        console.log('  Action: Waiting for nodes table to be visible');
        await page.locator('#nodes #nodes-table').waitFor({ state: 'visible', timeout: 10000 });
        console.log('  Action: Nodes table visible');
        // Optional: wait for rows as well, though table visibility might be enough
        // await page.locator('#nodes-table-body tr').first().waitFor({ state: 'visible', timeout: 10000 });
      }
    },

    // VM View: Click main tab, click VM filter, wait, capture main content
    { name: '03-vm-container-view', // Renaming slightly as it shows filtered list
      screenshotTarget: '#main',
      action: async (page) => {
        console.log('  Action: Clicking Main tab (if not already active)');
        // Ensure main tab is active first
        const mainTabIsActive = await page.locator('[data-tab="main"].active').isVisible();
        if (!mainTabIsActive) {
             await page.locator('[data-tab="main"]').click();
             await page.waitForLoadState('networkidle', { timeout: 5000 });
        }

        // Wait for the VM filter button's LABEL to be visible before clicking
        console.log('  Action: Waiting for VM filter label to be visible');
        const vmFilterLabel = page.locator('label[for="filter-vm"]'); // Target the label now
        await vmFilterLabel.waitFor({ state: 'visible', timeout: 15000 });
        console.log('  Action: VM filter label visible');

        console.log('  Action: Clicking VM filter label');
        await vmFilterLabel.click(); // Click the label
        await page.waitForTimeout(1000);
        console.log('  Action: VM filter applied');
      }
    },

    // PBS View: Click tab, wait for PBS container content, capture PBS tab content
    { name: '04-pbs-view',
      screenshotTarget: '#pbs',
      action: async (page) => {
        console.log('  Action: Clicking PBS tab');
        await page.locator('[data-tab="pbs"]').click();
        console.log('  Action: Waiting for PBS container to be visible');
        // Wait for the main container within the PBS tab to be visible
        await page.locator('#pbs #pbs-instances-container').waitFor({ state: 'visible', timeout: 10000 });
        console.log('  Action: PBS container visible');
         // Optional: wait for actual content rows if needed
        // await page.locator('#pbs-instances-container > *').first().waitFor({ state: 'visible', timeout: 10000 });
      }
    },

    // Backups View: Click tab, wait for table content, capture backups tab content
    { name: '05-backups-view', // Added new section
      screenshotTarget: '#backups',
      action: async (page) => {
        console.log('  Action: Clicking Backups tab');
        await page.locator('[data-tab="backups"]').click();
        console.log('  Action: Waiting for backups table row to be visible');
        // Wait for the first row in the backups table body
        await page.locator('#backups-overview-tbody tr').first().waitFor({ state: 'visible', timeout: 15000 }); 
        console.log('  Action: Backups table row visible');
      }
    },

    // Threshold & Logging View: Toggle thresholds, set one, start log, capture main area
    { name: '06-thresholds-logging', // New section
      screenshotTarget: '#main', // Capture the main dashboard/log area
      action: async (page) => {
        console.log('  Action: Ensuring Main tab is active');
        const mainTabIsActive = await page.locator('[data-tab="main"].active').isVisible();
        if (!mainTabIsActive) {
             await page.locator('[data-tab="main"]').click();
             await page.waitForLoadState('networkidle', { timeout: 5000 });
        }

        console.log('  Action: Ensuring guest type filter is set to \'All\'');
        const allFilterLabel = page.locator('label[for="filter-all"]');
        await allFilterLabel.waitFor({ state: 'visible', timeout: 5000 });
        await allFilterLabel.click();
        await page.waitForTimeout(500); // Allow filter UI to update

        console.log('  Action: Clicking Toggle Thresholds button');
        await page.locator('#toggle-thresholds-button').click();
        await page.locator('#threshold-slider-row').waitFor({ state: 'visible', timeout: 5000 });
        console.log('  Action: Setting CPU threshold slider (e.g., to 50%)');
        // Directly set the value for speed/reliability in automation
        await page.locator('#threshold-slider-cpu').fill('50');
        // Trigger input event manually after setting value programmatically
        await page.locator('#threshold-slider-cpu').dispatchEvent('input'); 
        await page.waitForTimeout(500); // Allow UI to react
        console.log('  Action: Waiting for Start Log button to be visible');
        await page.locator('#start-log-button:not(.hidden)').waitFor({ state: 'visible', timeout: 5000 });
        console.log('  Action: Clicking Start Log button');
        await page.locator('#start-log-button').click();
        console.log('  Action: Waiting for Log tab and content to appear');
        await page.locator('.nested-tab[data-nested-tab^="log-session-"]').waitFor({ state: 'visible', timeout: 10000 });
        await page.locator('.log-session-panel').waitFor({ state: 'visible', timeout: 10000 });
        console.log('  Action: Log session started and visible');
        console.log('  Action: Waiting 8 seconds for potential log entries...');
        await page.waitForTimeout(8000); // Increase pause to 8 seconds
      }
    },

    // { name: '06-task-view', url: '/#tasks', screenshotTarget: '#task-list-element', action: async (page) => { /* Navigate to task view if separate */ } }, // Uncomment and adjust if needed
];

async function takeScreenshots() {
    console.log(`Starting screenshot capture for ${BASE_URL}...`);
    console.log(`Outputting to: ${OUTPUT_DIR}`);

    // --- Clean up existing PNG files --- 
    if (fs.existsSync(OUTPUT_DIR)) {
        console.log(`Cleaning up existing *.png files in ${OUTPUT_DIR}...`);
        const files = fs.readdirSync(OUTPUT_DIR);
        let deletedCount = 0;
        files.forEach(file => {
            if (path.extname(file).toLowerCase() === '.png') {
                const filePath = path.join(OUTPUT_DIR, file);
                try {
                    fs.unlinkSync(filePath);
                    // console.log(`  Deleted: ${file}`); // Optional: more verbose logging
                    deletedCount++;
                } catch (err) {
                    console.error(`  Error deleting file ${file}: ${err.message}`);
                }
            }
        });
        console.log(`Cleanup finished. Deleted ${deletedCount} PNG file(s).`);
    } else {
        console.log('Output directory does not exist, no cleanup needed.');
    }
    // --- End Cleanup ---

    // Ensure output directory exists (might have been deleted if empty or just created)
    if (!fs.existsSync(OUTPUT_DIR)) {
        console.log(`Creating directory: ${OUTPUT_DIR}`);
        fs.mkdirSync(OUTPUT_DIR, { recursive: true });
    }

    let browser;
    try {
        browser = await chromium.launch(); // Or firefox, webkit
        const context = await browser.newContext({
            viewport: VIEWPORT,
            ignoreHTTPSErrors: true, // Helpful if using self-signed certs for Pulse dev
            deviceScaleFactor: 2 // Render at 2x detail for higher quality screenshots
        });
        const page = await context.newPage();

        console.log('Navigating to base URL and waiting for initial load...');
        await page.goto(BASE_URL, WAIT_OPTIONS);

        // Wait for the loading overlay to disappear before starting captures
        console.log(`Waiting for overlay (${OVERLAY_SELECTOR}) to disappear...`);
        await page.locator(OVERLAY_SELECTOR).waitFor({ state: 'hidden', timeout: 20000 }); // Increased timeout
        console.log('Overlay hidden.');

        // --- Ensure Dark Mode --- 
        console.log('Ensuring dark mode is active...');
        const isDarkMode = await page.evaluate(() => document.documentElement.classList.contains('dark'));
        if (!isDarkMode) {
            console.log(' Dark mode not active, clicking theme toggle button...');
            const themeButton = page.locator('#theme-toggle-button');
            await themeButton.waitFor({ state: 'visible', timeout: 5000 });
            await themeButton.click();
            await page.waitForTimeout(500); // Wait for theme transition
            console.log(' Dark mode toggled.');
        } else {
            console.log(' Dark mode already active.');
        }
        // --- End Ensure Dark Mode ---

        console.log('Starting section captures.');

        for (const section of sections) {
            const screenshotPath = path.join(OUTPUT_DIR, `${section.name}.png`);
            console.log(`Capturing section: ${section.name}...`);

            try {
                // Perform specific actions if needed (clicks, etc.)
                if (section.action) {
                    console.log(`  Performing action for ${section.name}...`);
                    await section.action(page);
                    // Wait after action for UI to settle - using networkidle should be sufficient
                    await page.waitForLoadState('networkidle', { timeout: 10000 }); 
                    console.log('  Action completed and network idle.');
                } else {
                    // Ensure we are on the main tab for sections without specific actions
                    // (Applies mainly if dashboard wasn't the very first step)
                    const mainTabIsActive = await page.locator('[data-tab="main"].active').isVisible();
                    if (!mainTabIsActive) {
                        await page.locator('[data-tab="main"]').click();
                        await page.waitForLoadState('networkidle', { timeout: 5000 });
                    }
                }

                // Take the screenshot
                let elementToCapture;
                let captureFullPage = section.fullPage || false; // Get flag, default false

                console.log(`  Locating screenshot target: ${section.screenshotTarget || 'page (fullPage: '+captureFullPage+')'}`);
                if (section.screenshotTarget) { // Check if a specific target is defined
                    elementToCapture = page.locator(section.screenshotTarget).first();
                    console.log('  Waiting for screenshot target element to be visible...');
                    await elementToCapture.waitFor({ state: 'visible', timeout: 10000 });
                    console.log('  Target element visible.');
                    captureFullPage = false; // Never capture full page when targeting a specific element
                } else {
                    elementToCapture = page; // Use the page itself for viewport/fullpage screenshots
                }

                console.log(`  Saving screenshot to: ${screenshotPath}`);
                if (elementToCapture === page) {
                    // Capture viewport or full page based on the flag
                    console.log(`  Capturing ${captureFullPage ? 'full page' : 'viewport'}`);
                    await page.screenshot({ path: screenshotPath, fullPage: captureFullPage });
                } else {
                    // Capture specific element
                     console.log('  Capturing specific element');
                    await elementToCapture.screenshot({ path: screenshotPath });
                }

                console.log(`  Successfully captured ${section.name}`);

            } catch (error) {
                console.error(`  Failed to capture section ${section.name}: ${error.message}`);
                // Optionally, decide if you want to continue or stop on error
            }
        }

    } catch (error) {
        console.error(`Error during screenshot process: ${error}`);
        process.exitCode = 1; // Indicate failure
    } finally {
        if (browser) {
            await browser.close();
            console.log('Browser closed.');
        }
    }

    console.log('Screenshot capture finished.');
}

takeScreenshots(); 