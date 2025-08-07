const { chromium } = require('playwright');

const PULSE_URL = process.env.PULSE_URL || 'http://192.168.0.212:7655';

async function testNavigation() {
  console.log('🚀 Testing Pulse Navigation');
  console.log(`📍 URL: ${PULSE_URL}`);
  
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  
  try {
    // Load main page
    await page.goto(PULSE_URL);
    await page.waitForLoadState('networkidle');
    
    // Find all navigation links
    console.log('\n📋 Navigation Links:');
    const navLinks = await page.locator('a, button').all();
    
    for (const link of navLinks) {
      const text = await link.textContent();
      const href = await link.getAttribute('href') || '';
      if (text && text.trim()) {
        console.log(`  - "${text.trim()}" ${href ? `(${href})` : ''}`);
      }
    }
    
    // Click Settings
    console.log('\n⚙️ Clicking Settings...');
    await page.click('text=Settings');
    await page.waitForTimeout(1000);
    
    // Check URL
    const url = page.url();
    console.log(`📍 Current URL: ${url}`);
    
    // Find all tabs/sections on settings page
    console.log('\n📋 Settings Page Sections:');
    const sections = await page.locator('h1, h2, h3, button, a').all();
    
    for (const section of sections) {
      const text = await section.textContent();
      if (text && text.trim() && !text.includes('Toggle') && !text.includes('Mode')) {
        console.log(`  - "${text.trim()}"`);
      }
    }
    
    // Look specifically for System/Updates
    console.log('\n🔍 Looking for System/Updates sections...');
    
    // Try clicking on different tabs
    const tabs = ['System', 'Configuration', 'General', 'Security'];
    for (const tab of tabs) {
      const tabElement = page.locator(`text="${tab}"`).first();
      if (await tabElement.count() > 0) {
        console.log(`✅ Found "${tab}" tab, clicking...`);
        await tabElement.click();
        await page.waitForTimeout(500);
        
        // Check for Updates after clicking
        const pageContent = await page.textContent('body');
        if (pageContent.includes('Update') || pageContent.includes('Version')) {
          console.log(`  ✅ Found Update/Version content in ${tab} tab!`);
          
          // Take screenshot
          await page.screenshot({ path: `screenshots/tab-${tab.toLowerCase()}.png`, fullPage: true });
        }
      }
    }
    
  } catch (error) {
    console.error('❌ Error:', error.message);
  } finally {
    await browser.close();
  }
}

testNavigation().catch(console.error);