const { chromium } = require('playwright');
const axios = require('axios');

async function testButtonFunctionality() {
  const browser = await chromium.launch({ 
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });
  
  const context = await browser.newContext();
  const page = await context.newPage();
  
  const results = {
    passed: [],
    failed: []
  };
  
  function logTest(name, passed, details = '') {
    console.log(`${passed ? '✅' : '❌'} ${name}${details ? ' - ' + details : ''}`);
    if (passed) results.passed.push(name);
    else results.failed.push({ name, details });
  }
  
  try {
    console.log('=== TESTING BUTTON FUNCTIONALITY ===\n');
    
    // Navigate to Pulse
    await page.goto('http://localhost:7655');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);
    
    // 1. TEST NAVIGATION BUTTONS
    console.log('1. NAVIGATION BUTTONS');
    console.log('   ------------------');
    
    const tabs = ['Storage', 'Backups', 'Alerts', 'Settings'];
    for (const tab of tabs) {
      await page.locator(`div[role="tab"]:has-text("${tab}")`).click();
      await page.waitForTimeout(500);
      const hasContent = await page.locator(`text=/${tab}/i`).first().isVisible();
      logTest(`${tab} tab navigation`, hasContent);
    }
    
    // 2. TEST SAVE/CANCEL BUTTONS
    console.log('\n2. SAVE/CANCEL BUTTONS');
    console.log('   --------------------');
    
    await page.locator('div[role="tab"]:has-text("Alerts")').click();
    await page.waitForTimeout(500);
    await page.locator('button:has-text("Thresholds")').click();
    await page.waitForTimeout(500);
    
    // Change a threshold
    const slider = page.locator('input[type="range"]').first();
    const originalValue = await slider.inputValue();
    await slider.fill(originalValue === '80' ? '85' : '80');
    await page.waitForTimeout(500);
    
    // Check for save button
    const saveVisible = await page.locator('button:has-text("Save Changes")').isVisible({ timeout: 3000 }).catch(() => false);
    logTest('Save Changes button appears', saveVisible);
    
    if (saveVisible) {
      await page.locator('button:has-text("Save Changes")').click();
      const saved = await page.locator('text=Configuration saved successfully').waitFor({ timeout: 5000 }).then(() => true).catch(() => false);
      logTest('Save Changes works', saved);
    }
    
    // 3. TEST ALERT BUTTONS
    console.log('\n3. ALERT ACTION BUTTONS');
    console.log('   ---------------------');
    
    await page.locator('button:has-text("Overview")').click();
    await page.waitForTimeout(500);
    
    const hasAckButton = await page.locator('button:has-text("Acknowledge")').isVisible().catch(() => false);
    const hasClearButton = await page.locator('button:has-text("Clear")').isVisible().catch(() => false);
    
    logTest('Alert buttons present', hasAckButton || hasClearButton, `Ack: ${hasAckButton}, Clear: ${hasClearButton}`);
    
    // Summary
    console.log('\n' + '='.repeat(50));
    console.log('BUTTON TEST SUMMARY');
    console.log('='.repeat(50));
    console.log(`Total Tests: ${results.passed.length + results.failed.length}`);
    console.log(`Passed: ${results.passed.length} ✅`);
    console.log(`Failed: ${results.failed.length} ❌`);
    
    if (results.failed.length > 0) {
      console.log('\nFailed Tests:');
      results.failed.forEach(f => console.log(`  - ${f.name}: ${f.details}`));
    }
    
  } catch (error) {
    console.error('\nTest error:', error.message);
    await page.screenshot({ path: 'error-button-test.png' });
  } finally {
    await browser.close();
  }
}

// Run the test
if (require.main === module) {
  testButtonFunctionality().catch(console.error);
}

module.exports = { testButtonFunctionality };