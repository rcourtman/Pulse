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

    // PBS View: Click tab, wait for PBS container content, capture PBS tab content
    { name: '02-pbs-view', // Renumbered from 03
      screenshotTarget: '#pbs',
      action: async (page) => {
        console.log('  Action: Clicking PBS tab');
        await page.locator('[data-tab="pbs"]').click();
        console.log('  Action: Waiting for PBS container to be visible');
        // Wait for the main container within the PBS tab to be visible
        await page.locator('#pbs #pbs-instances-container').waitFor({ state: 'visible', timeout: 10000 });
        console.log('  Action: PBS container visible');
        
        // Wait for PBS data to load - look for actual content, not loading message
        console.log('  Action: Waiting for PBS data to load');
        try {
          // Wait for either a PBS table row or status content to appear
          await page.locator('#pbs .pbs-status-table tbody tr, #pbs .pbs-datastore-table tbody tr').first().waitFor({ state: 'visible', timeout: 15000 });
          console.log('  Action: PBS data loaded');
        } catch (e) {
          console.log('  Warning: PBS data may not be fully loaded');
        }
        
        // Additional wait to ensure all data is rendered
        await page.waitForTimeout(1000);
      }
    },

    // Backups View: Click tab, wait for table content, capture backups tab content
    { name: '03-backups-view', // Renumbered from 04
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
    
    // Line Graph Toggle View: Click the charts toggle button to show charts
    { name: '04-line-graph-toggle', 
      screenshotTarget: '#nested-tab-dashboard',
      action: async (page) => {
        console.log('  Action: Ensuring Main tab is active');
        // Ensure main tab is active
        const mainTabIsActive = await page.locator('[data-tab="main"].active').isVisible();
        if (!mainTabIsActive) {
             await page.locator('[data-tab="main"]').click();
             await page.waitForLoadState('networkidle', { timeout: 5000 });
        }

        // Hide node summary cards
        console.log('  Action: Hiding node summary cards');
        await page.locator('#node-summary-cards-container').evaluate(element => element.style.display = 'none');

        // Filter to show only LXC containers
        console.log('  Action: Clicking LXC filter');
        const lxcFilterLabel = page.locator('label[for="filter-lxc"]');
        await lxcFilterLabel.waitFor({ state: 'visible', timeout: 10000 });
        await lxcFilterLabel.click();
        await page.waitForTimeout(500);

        console.log('  Action: Clicking charts toggle button');
        // Click the charts toggle button
        const chartsToggle = page.locator('#toggle-charts-button');
        await chartsToggle.waitFor({ state: 'visible', timeout: 10000 });
        await chartsToggle.click();
        
        console.log('  Action: Waiting for charts to appear');
        // Wait for charts to be visible
        await page.waitForTimeout(2000); // Allow time for charts to render and data to load
        
        // Wait for the main container to have charts-mode class indicating charts are shown
        console.log('  Action: Checking for charts mode');
        await page.waitForFunction(() => {
            const mainContainer = document.getElementById('main');
            return mainContainer && mainContainer.classList.contains('charts-mode');
        }, { timeout: 5000 });
        
        // Additional wait to ensure charts are fully rendered
        await page.waitForTimeout(2000);
        console.log('  Action: Charts are now visible');
        
        // Hover over a chart to show tooltip
        console.log('  Action: Hovering over a chart to show tooltip');
        try {
            // Find the first visible chart element
            const firstChart = page.locator('[id^="chart-"][id$="-cpu"] svg').first();
            await firstChart.waitFor({ state: 'visible', timeout: 5000 });
            
            // Get the bounding box of the chart
            const box = await firstChart.boundingBox();
            if (box) {
                // Move mouse to middle-right of the chart (where recent data points are)
                const hoverX = box.x + (box.width * 0.8);
                const hoverY = box.y + (box.height * 0.5);
                
                await page.mouse.move(hoverX, hoverY);
                
                // Wait for tooltip to appear
                await page.waitForTimeout(500);
                console.log('  Action: Tooltip should now be visible');
            }
        } catch (e) {
            console.log('  Warning: Could not hover over chart for tooltip');
        }
      },
      postAction: async (page) => {
        console.log('  Action: Clicking charts toggle button again to hide charts');
        // Toggle charts off again
        const chartsToggle = page.locator('#toggle-charts-button');
        await chartsToggle.click();
        await page.waitForTimeout(500);
        
        // Reset filter to show all
        console.log('  Action: Resetting filter to show all');
        const allFilterLabel = page.locator('label[for="filter-all"]');
        await allFilterLabel.click();
        await page.waitForTimeout(500);
        
        // Show node summary cards again
        console.log('  Action: Showing node summary cards');
        await page.locator('#node-summary-cards-container').evaluate(element => element.style.display = '');
      }
    }
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

                // Perform post-action if defined (e.g., to restore UI state)
                if (section.postAction) {
                    console.log(`  Performing post-action for ${section.name}...`);
                    await section.postAction(page);
                    // Optionally wait for UI to settle after post-action
                    await page.waitForLoadState('networkidle', { timeout: 5000 }); 
                    console.log('  Post-action completed and network idle.');
                }

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