const { chromium } = require('playwright');

const FRONTEND_URL = 'http://192.168.0.123:7655';

async function testMobileStorageView() {
  console.log('=== TESTING STORAGE TAB MOBILE RESPONSIVENESS ===\n');
  
  const browser = await chromium.launch({ headless: true });
  
  try {
    // Test different viewport sizes
    const viewports = [
      { name: 'Mobile (iPhone 12)', width: 390, height: 844, device: 'mobile' },
      { name: 'Small Tablet', width: 640, height: 768, device: 'sm' },
      { name: 'Medium Tablet', width: 768, height: 1024, device: 'md' },
      { name: 'Large Tablet', width: 1024, height: 768, device: 'lg' },
      { name: 'Desktop', width: 1280, height: 800, device: 'desktop' }
    ];
    
    for (const viewport of viewports) {
      console.log(`\nTesting ${viewport.name} (${viewport.width}x${viewport.height}):`);
      console.log('  ' + '-'.repeat(40));
      
      const context = await browser.newContext({
        viewport: { width: viewport.width, height: viewport.height }
      });
      const page = await context.newPage();
      
      // Navigate to frontend
      await page.goto(FRONTEND_URL);
      await page.waitForTimeout(1000); // Wait for initial load
      
      // Click on Storage tab
      await page.click('div[role="tab"]:has-text("Storage")');
      await page.waitForTimeout(1000); // Wait for tab to load
      
      // Wait for storage table specifically  
      await page.waitForSelector('table', { timeout: 10000 });
      
      // Check visible columns
      const visibleColumns = await page.evaluate(() => {
        const headers = Array.from(document.querySelectorAll('thead th'));
        return headers
          .filter(th => {
            const style = window.getComputedStyle(th);
            return style.display !== 'none' && style.visibility !== 'hidden';
          })
          .map(th => th.textContent.trim());
      });
      
      console.log('  Visible columns:', visibleColumns.join(', '));
      
      // Check if table is scrollable
      const isScrollable = await page.evaluate(() => {
        const container = document.querySelector('.overflow-x-auto');
        return container ? container.scrollWidth > container.clientWidth : false;
      });
      
      console.log('  Table scrollable:', isScrollable ? 'Yes' : 'No');
      
      // Check if storage data is loaded
      const hasStorageData = await page.evaluate(() => {
        const rows = document.querySelectorAll('tbody tr');
        return rows.length > 0;
      });
      
      if (hasStorageData) {
        // Check usage bar content
        const usageBarContent = await page.evaluate(() => {
          const progressBar = document.querySelector('td div[class*="relative w-full"] span.truncate');
          if (!progressBar) return 'Not found';
          
          // Check which variant is visible
          const shortVersion = progressBar.querySelector('.sm\\:hidden');
          const fullVersion = progressBar.querySelector('.hidden.sm\\:inline');
          
          if (shortVersion && window.getComputedStyle(shortVersion).display !== 'none') {
            return `Short: ${shortVersion.textContent.trim()}`;
          } else if (fullVersion && window.getComputedStyle(fullVersion).display !== 'none') {
            return `Full: ${fullVersion.textContent.trim()}`;
          }
          
          return 'No content visible';
        });
        
        console.log('  Usage bar shows:', usageBarContent);
      } else {
        console.log('  No storage data loaded');
      }
      
      // Take screenshot
      await page.screenshot({ 
        path: `storage-${viewport.device}.png`,
        fullPage: false 
      });
      console.log(`  Screenshot saved: storage-${viewport.device}.png`);
      
      await context.close();
    }
    
    console.log('\n✅ Mobile responsiveness test complete!');
    console.log('Check the screenshot files to visually verify the layout.');
    
  } catch (error) {
    console.error('❌ Test failed:', error.message);
  } finally {
    await browser.close();
  }
}

// Run the test
if (require.main === module) {
  testMobileStorageView().catch(console.error);
}

module.exports = { testMobileStorageView };