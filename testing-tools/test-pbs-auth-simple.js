const { chromium } = require('playwright');

const BASE_URL = 'http://192.168.0.123:7655';
const PBS_HOST = 'https://192.168.0.8:8007';

// PBS credentials
const TOKEN_AUTH = {
  tokenName: 'pulse-monitor@pbs!pulse-token',
  tokenValue: 'c5d5bf2a-35a0-4c82-bdaf-a052c10dedd6'
};

const PASSWORD_AUTH = {
  user: 'admin@pbs',
  password: '1b9edcfc7e'
};

async function testPBSAuth() {
  const browser = await chromium.launch({ 
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });
  
  const page = await browser.newContext({ ignoreHTTPSErrors: true })
    .then(ctx => ctx.newPage());
  
  try {
    console.log('Navigating to Pulse...');
    await page.goto(BASE_URL);
    await page.waitForTimeout(3000);
    
    // Take initial screenshot
    await page.screenshot({ path: 'pbs-test-1-home.png' });
    console.log('Screenshot 1: Home page');
    
    // Click on Settings - try different approaches
    console.log('\nTrying to navigate to Settings...');
    
    // Method 1: Click visible text
    const settingsVisible = await page.locator('text=Settings').isVisible();
    console.log('Settings text visible:', settingsVisible);
    
    if (settingsVisible) {
      await page.locator('text=Settings').first().click();
      await page.waitForTimeout(2000);
    }
    
    // Take screenshot after clicking settings
    await page.screenshot({ path: 'pbs-test-2-settings.png' });
    console.log('Screenshot 2: After clicking Settings');
    
    // Check what's visible on the page
    const pageContent = await page.locator('body').innerText();
    console.log('\nPage contains "Proxmox":', pageContent.includes('Proxmox'));
    console.log('Page contains "PBS":', pageContent.includes('PBS'));
    console.log('Page contains "Add":', pageContent.includes('Add'));
    
    // Look for Add PBS button variations
    const addButtons = [
      'button:text("Add PBS Node")',
      'button:text("Add PBS")',
      'button:text("+ Add PBS")',
      'button:text("PBS")',
      'text=Add PBS'
    ];
    
    console.log('\nLooking for Add PBS button...');
    for (const selector of addButtons) {
      const isVisible = await page.locator(selector).isVisible();
      console.log(`  ${selector}: ${isVisible}`);
      if (isVisible) {
        console.log(`  Found button with selector: ${selector}`);
        break;
      }
    }
    
    // Get all buttons on the page
    const buttons = await page.locator('button').allTextContents();
    console.log('\nAll buttons on page:', buttons);
    
  } catch (error) {
    console.error('Error:', error.message);
    await page.screenshot({ path: 'pbs-test-error.png' });
  } finally {
    await browser.close();
  }
}

testPBSAuth().catch(console.error);