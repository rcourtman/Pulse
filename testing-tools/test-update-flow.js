const { chromium } = require('playwright');

const PULSE_URL = process.env.PULSE_URL || 'http://192.168.0.212:7655';

async function testUpdateFlow() {
  console.log('🚀 Starting Update Flow Test');
  console.log(`📍 Testing against: ${PULSE_URL}`);
  
  const browser = await chromium.launch({ 
    headless: true, // Run headless since no display
    slowMo: 100 // Slight delay to avoid race conditions
  });
  
  const context = await browser.newContext({
    viewport: { width: 1280, height: 720 },
    recordVideo: {
      dir: './test-videos/',
      size: { width: 1280, height: 720 }
    }
  });
  
  const page = await context.newPage();
  
  // Enable console logging
  page.on('console', msg => {
    if (msg.type() === 'error') {
      console.log('❌ Browser console error:', msg.text());
    }
  });
  
  try {
    // Step 1: Navigate to Pulse
    console.log('📄 Loading Pulse dashboard...');
    await page.goto(PULSE_URL);
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: 'screenshots/01-dashboard.png' });
    
    // Step 2: Check current version
    const versionText = await page.textContent('text=/Version.*\\d+\\.\\d+\\.\\d+/');
    console.log(`📌 Current version: ${versionText}`);
    
    // Step 3: Navigate to Settings -> System
    console.log('⚙️ Navigating to Settings...');
    
    // First click Settings (sidebar doesn't exist in v4.0.10, it's in the header)
    await page.click('text=Settings');
    await page.waitForTimeout(1000);
    await page.screenshot({ path: 'screenshots/02-settings-page.png' });
    
    // Now click System tab
    console.log('📍 Clicking System tab...');
    await page.click('button:has-text("System")');
    await page.waitForTimeout(2000);
    
    // Debug: Check what content is visible
    const systemContent = await page.textContent('body');
    if (systemContent.includes('Performance')) {
      console.log('✅ Found Performance section');
    }
    if (systemContent.includes('Updates')) {
      console.log('✅ Found Updates section in content');
    }
    
    await page.screenshot({ path: 'screenshots/03-system-tab.png' });
    
    // Step 4: Look for Updates section
    console.log('🔍 Looking for Updates section...');
    
    // Debug: Get all text on settings page
    const pageText = await page.textContent('body');
    if (pageText.includes('Update')) {
      console.log('✅ Found "Update" text somewhere on page');
    }
    
    // Try multiple selectors
    const updateSelectors = [
      'text=Updates',
      'text=Update',
      'h3:has-text("Updates")',
      'div:has-text("Updates")',
      'text=Current Version',
      'text=Check for Updates'
    ];
    
    let updatesSection = null;
    for (const selector of updateSelectors) {
      const element = page.locator(selector).first();
      if (await element.count() > 0) {
        console.log(`✅ Found element with selector: ${selector}`);
        updatesSection = element;
        break;
      }
    }
    
    if (updatesSection && await updatesSection.isVisible()) {
      console.log('✅ Found Updates section');
      await updatesSection.scrollIntoViewIfNeeded();
      await page.screenshot({ path: 'screenshots/03-updates-section.png' });
    } else {
      console.log('❌ Updates section not found!');
      console.log('📋 Taking full page screenshot for debugging...');
      await page.screenshot({ path: 'screenshots/debug-settings-page.png', fullPage: true });
      
      // Debug: List all visible headings
      const headings = await page.locator('h2, h3, h4').all();
      console.log(`Found ${headings.length} headings:`);
      for (const heading of headings) {
        const text = await heading.textContent();
        console.log(`  - ${text}`);
      }
    }
    
    // Step 5: Check for "Check for Updates" button
    console.log('🔍 Looking for Check for Updates button...');
    const checkButton = page.locator('button:has-text("Check for Updates")');
    if (await checkButton.isVisible()) {
      console.log('✅ Found Check for Updates button');
      await checkButton.click();
      console.log('⏳ Checking for updates...');
      await page.waitForTimeout(3000);
      await page.screenshot({ path: 'screenshots/04-after-check.png' });
    } else {
      console.log('❓ No Check for Updates button, checking if update already detected...');
    }
    
    // Step 6: Look for update available message
    console.log('🔍 Looking for update available notification...');
    const updateAvailable = await page.locator('text=/Update.*[Aa]vailable.*v?\\d+\\.\\d+\\.\\d+/').first();
    if (await updateAvailable.isVisible()) {
      const updateText = await updateAvailable.textContent();
      console.log(`✅ Update available: ${updateText}`);
      await page.screenshot({ path: 'screenshots/05-update-available.png' });
    } else {
      console.log('❌ No update available message found');
      // Take debug screenshot
      await page.screenshot({ path: 'screenshots/debug-no-update.png', fullPage: true });
    }
    
    // Step 7: Look for Apply Update button
    console.log('🔍 Looking for Apply Update button...');
    const applyButton = page.locator('button:has-text("Apply Update")');
    
    // Debug: Check all buttons in the Updates section
    const allButtons = await page.locator('button').all();
    console.log(`📊 Found ${allButtons.length} total buttons on page`);
    
    for (const button of allButtons) {
      const text = await button.textContent();
      const isVisible = await button.isVisible();
      if (text && text.includes('Update')) {
        console.log(`  - Button: "${text}" (visible: ${isVisible})`);
      }
    }
    
    if (await applyButton.count() > 0) {
      console.log(`✅ Found ${await applyButton.count()} Apply Update button(s)`);
      const isVisible = await applyButton.first().isVisible();
      console.log(`   Visible: ${isVisible}`);
      
      if (isVisible) {
        console.log('🎯 Apply Update button is visible!');
        await applyButton.first().scrollIntoViewIfNeeded();
        await page.screenshot({ path: 'screenshots/06-apply-button.png' });
        
        // Step 8: Click Apply Update
        console.log('🚀 Clicking Apply Update...');
        await applyButton.first().click();
        await page.waitForTimeout(2000);
        await page.screenshot({ path: 'screenshots/07-after-apply.png' });
        
        // Step 9: Check for progress or status
        console.log('⏳ Checking update status...');
        const statusMessages = [
          'text=/[Uu]pdat.*progress/',
          'text=/[Dd]ownload/',
          'text=/[Ii]nstall/',
          'text=/[Rr]estart/'
        ];
        
        for (const selector of statusMessages) {
          const element = page.locator(selector).first();
          if (await element.isVisible()) {
            const text = await element.textContent();
            console.log(`📊 Status: ${text}`);
          }
        }
      } else {
        console.log('❌ Apply Update button exists but is NOT visible');
        
        // Debug: Check parent visibility
        const parent = await applyButton.first().locator('..');
        const parentVisible = await parent.isVisible();
        console.log(`   Parent visible: ${parentVisible}`);
        
        // Debug: Get computed styles
        const styles = await applyButton.first().evaluate(el => {
          const computed = window.getComputedStyle(el);
          return {
            display: computed.display,
            visibility: computed.visibility,
            opacity: computed.opacity,
            position: computed.position
          };
        });
        console.log('   Computed styles:', styles);
      }
    } else {
      console.log('❌ No Apply Update button found at all');
    }
    
    // Step 10: Debug - Get the entire Updates section HTML
    console.log('\n📋 Debug: Updates section HTML structure');
    const updatesContainer = page.locator('text=Updates').first().locator('..');
    const updatesHTML = await updatesContainer.innerHTML();
    
    // Look for any update-related elements
    if (updatesHTML.includes('4.0.11') || updatesHTML.includes('4.0.12')) {
      console.log('✅ Found version references in HTML');
    }
    if (updatesHTML.includes('Apply')) {
      console.log('✅ Found "Apply" text in HTML');
    }
    if (updatesHTML.includes('button')) {
      console.log('✅ Found button elements in HTML');
    }
    
    // Save HTML for inspection
    require('fs').writeFileSync('debug-updates-section.html', updatesHTML);
    console.log('💾 Saved Updates section HTML to debug-updates-section.html');
    
    // Final screenshot
    await page.screenshot({ path: 'screenshots/08-final-state.png', fullPage: true });
    
  } catch (error) {
    console.error('❌ Test failed:', error);
    await page.screenshot({ path: 'screenshots/error.png', fullPage: true });
  } finally {
    // Keep browser open for manual inspection
    console.log('\n✅ Test complete. Browser will stay open for 30 seconds for inspection...');
    await page.waitForTimeout(30000);
    
    await context.close();
    await browser.close();
  }
}

// Run the test
testUpdateFlow().catch(console.error);