const { chromium } = require('playwright');

const FRONTEND_URL = 'http://192.168.0.123:7655';

async function testMobileDashboardView() {
  console.log('=== TESTING DASHBOARD MOBILE RESPONSIVENESS ===\n');
  
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
      
      // Navigate to frontend (main dashboard is default)
      await page.goto(FRONTEND_URL);
      await page.waitForTimeout(2000); // Wait for data to load
      
      // Wait for content to load (either table or cards)
      await page.waitForSelector('.bg-white.dark\\:bg-gray-800, table', { timeout: 10000 });
      
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
      
      // Check if using cards or table
      const isCardView = await page.evaluate(() => {
        const cardContainer = document.querySelector('.block.sm\\:hidden');
        return cardContainer && window.getComputedStyle(cardContainer).display !== 'none';
      });
      
      if (isCardView) {
        // Count cards
        const cardCount = await page.evaluate(() => {
          return document.querySelectorAll('.bg-white.dark\\:bg-gray-800.border.border-gray-200').length;
        });
        console.log('  View type: Card layout');
        console.log('  Cards visible:', cardCount);
        
        // Check card content
        const firstCardInfo = await page.evaluate(() => {
          const cards = Array.from(document.querySelectorAll('.bg-white.dark\\:bg-gray-800.border.border-gray-200.rounded-lg.p-3'));
          if (cards.length === 0) return 'No cards found';
          
          const firstCard = cards[0];
          const name = firstCard.querySelector('h3')?.textContent || 'No name';
          const type = firstCard.querySelector('.rounded.font-medium')?.textContent || '';
          const vmid = firstCard.querySelector('.text-gray-600.dark\\:text-gray-400')?.textContent || '';
          
          const stats = Array.from(firstCard.querySelectorAll('.space-y-1 > div'))
            .map(row => {
              const label = row.querySelector('.text-gray-600')?.textContent || '';
              const value = row.querySelector('.font-medium')?.textContent?.trim() || '';
              return `${label}:${value}`;
            })
            .slice(0, 3)
            .join(', ');
          
          return `${name} ${type} ${vmid} - ${stats}`;
        });
        console.log('  First card info:', firstCardInfo);
      } else {
        console.log('  View type: Table layout');
      }
      
      // Check row count
      const rowCount = await page.evaluate(() => {
        return document.querySelectorAll('tbody tr').length;
      });
      
      console.log('  Guests visible:', rowCount);
      
      // Take screenshot
      await page.screenshot({ 
        path: `dashboard-${viewport.device}.png`,
        fullPage: false 
      });
      console.log(`  Screenshot saved: dashboard-${viewport.device}.png`);
      
      await context.close();
    }
    
    console.log('\n✅ Dashboard mobile responsiveness test complete!');
    console.log('Check the screenshot files to visually verify the layout.');
    
  } catch (error) {
    console.error('❌ Test failed:', error.message);
  } finally {
    await browser.close();
  }
}

// Run the test
if (require.main === module) {
  testMobileDashboardView().catch(console.error);
}

module.exports = { testMobileDashboardView };