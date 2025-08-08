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

async function delay(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function testPBSAuthentication() {
  const browser = await chromium.launch({ 
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });
  
  const context = await browser.newContext({
    ignoreHTTPSErrors: true,
    viewport: { width: 1920, height: 1080 }
  });
  
  const page = await context.newPage();
  
  try {
    console.log('🚀 Starting PBS Authentication Tests...\n');
    
    // Navigate to Pulse
    console.log('📍 Navigating to Pulse...');
    await page.goto(BASE_URL);
    await page.waitForLoadState('networkidle');
    await delay(2000);
    
    // Go to Settings page
    console.log('⚙️ Opening Settings page...');
    
    // Navigate directly to settings since it's a SPA
    await page.evaluate(() => {
      const settingsLink = document.querySelector('a[href="/settings"], button:has-text("Settings")');
      if (settingsLink) settingsLink.click();
    });
    
    await delay(2000);
    
    // Check if we're on settings page
    const onSettings = await page.locator('text=/Proxmox|System Settings|Node Management/i').isVisible({ timeout: 2000 });
    if (!onSettings) {
      console.log('   Navigating directly to settings URL...');
      await page.goto(BASE_URL + '/#/settings');
      await delay(2000);
    }
    
    // First, remove any existing PBS nodes
    console.log('🧹 Cleaning up existing PBS nodes...');
    const pbsCards = await page.locator('.bg-white:has-text("PBS:")').all();
    for (const card of pbsCards) {
      const deleteButton = card.locator('button[title="Delete node"]');
      if (await deleteButton.isVisible()) {
        await deleteButton.click();
        await delay(500);
        // Confirm deletion
        const confirmButton = page.locator('button:has-text("Delete"):not([title])');
        if (await confirmButton.isVisible()) {
          await confirmButton.click();
          await delay(1000);
        }
      }
    }
    
    // TEST 1: Token-based authentication
    console.log('\n=== TEST 1: Token-based Authentication ===\n');
    
    console.log('➕ Adding PBS node with token authentication...');
    
    // Debug: Check what's on the page
    await page.screenshot({ path: 'pbs-auth-debug-settings.png', fullPage: true });
    console.log('   Debug screenshot saved to pbs-auth-debug-settings.png');
    
    // Try different selectors for Add PBS button
    let addPBSButton = page.locator('button:has-text("Add PBS Node")');
    if (!await addPBSButton.isVisible({ timeout: 2000 })) {
      addPBSButton = page.locator('button:has-text("Add PBS")');
    }
    if (!await addPBSButton.isVisible({ timeout: 2000 })) {
      addPBSButton = page.locator('button').filter({ hasText: /PBS/i });
    }
    
    await addPBSButton.click();
    await page.waitForSelector('h3:has-text("Add PBS Node")', { timeout: 5000 });
    
    // Fill in token auth details
    await page.fill('input[placeholder="Enter node name (optional)"]', 'PBS-Token');
    await page.fill('input[placeholder="https://pbs.example.com:8007"]', PBS_HOST);
    
    // Switch to token auth
    const tokenRadio = page.locator('label:has-text("API Token")');
    await tokenRadio.click();
    await delay(500);
    
    // Fill token fields
    await page.fill('input[placeholder="user@realm!tokenname"]', TOKEN_AUTH.tokenName);
    await page.fill('input[placeholder="Enter token value"]', TOKEN_AUTH.tokenValue);
    
    // Test connection
    console.log('🔌 Testing token connection...');
    await page.locator('button:has-text("Test Connection")').click();
    await delay(3000);
    
    // Check for success message
    const tokenTestResult = await page.locator('.text-green-600, .text-green-500, .text-green-700').first();
    if (await tokenTestResult.isVisible()) {
      const resultText = await tokenTestResult.textContent();
      console.log('✅ Token connection test: ' + resultText);
    } else {
      console.log('❌ Token connection test failed');
    }
    
    // Save the node
    await page.locator('button:has-text("Add Node")').click();
    await delay(2000);
    
    // Verify the node shows "Token:" in the display
    console.log('🔍 Verifying token auth display...');
    const tokenNodeCard = page.locator('.bg-white:has-text("PBS-Token")').first();
    const tokenAuthDisplay = await tokenNodeCard.locator('span:has-text("Token:")').textContent();
    console.log('   Display shows: ' + tokenAuthDisplay);
    
    if (tokenAuthDisplay.includes('Token:')) {
      console.log('✅ Token authentication display is correct');
    } else {
      console.log('❌ Token authentication display is incorrect (shows User instead of Token)');
    }
    
    // TEST 2: Edit token node and verify it stays as token
    console.log('\n📝 Testing edit mode for token node...');
    await tokenNodeCard.locator('button[title="Edit node"]').click();
    await page.waitForSelector('h3:has-text("Edit PBS Node")', { timeout: 5000 });
    await delay(1000);
    
    // Check if token auth is selected
    const tokenRadioChecked = await page.locator('input[type="radio"][value="token"]').isChecked();
    if (tokenRadioChecked) {
      console.log('✅ Edit mode correctly shows token authentication selected');
    } else {
      console.log('❌ Edit mode incorrectly shows password authentication selected');
    }
    
    // Cancel edit
    await page.locator('button:has-text("Cancel")').click();
    await delay(1000);
    
    // TEST 3: Password-based authentication
    console.log('\n=== TEST 2: Password-based Authentication ===\n');
    
    console.log('➕ Adding PBS node with password authentication...');
    await page.locator('button:has-text("Add PBS Node")').click();
    await page.waitForSelector('h3:has-text("Add PBS Node")', { timeout: 5000 });
    
    // Fill in password auth details
    await page.fill('input[placeholder="Enter node name (optional)"]', 'PBS-Password');
    await page.fill('input[placeholder="https://pbs.example.com:8007"]', PBS_HOST);
    
    // Make sure password auth is selected (default)
    const passwordRadio = page.locator('label:has-text("Username/Password")');
    await passwordRadio.click();
    await delay(500);
    
    // Fill credentials
    await page.fill('input[placeholder="user@realm"]', PASSWORD_AUTH.user);
    await page.fill('input[placeholder="Enter password"]', PASSWORD_AUTH.password);
    
    // Test connection
    console.log('🔌 Testing password connection...');
    await page.locator('button:has-text("Test Connection")').click();
    await delay(3000);
    
    // Check for success message
    const passwordTestResult = await page.locator('.text-green-600, .text-green-500, .text-green-700').first();
    if (await passwordTestResult.isVisible()) {
      const resultText = await passwordTestResult.textContent();
      console.log('✅ Password connection test: ' + resultText);
    } else {
      console.log('❌ Password connection test failed');
    }
    
    // Save the node
    await page.locator('button:has-text("Add Node")').click();
    await delay(2000);
    
    // Verify the node shows "User:" in the display
    console.log('🔍 Verifying password auth display...');
    const passwordNodeCard = page.locator('.bg-white:has-text("PBS-Password")').first();
    const passwordAuthDisplay = await passwordNodeCard.locator('span:has-text("User:")').textContent();
    console.log('   Display shows: ' + passwordAuthDisplay);
    
    if (passwordAuthDisplay.includes('User:')) {
      console.log('✅ Password authentication display is correct');
    } else {
      console.log('❌ Password authentication display is incorrect (shows Token instead of User)');
    }
    
    // TEST 4: Edit password node and verify it stays as password
    console.log('\n📝 Testing edit mode for password node...');
    await passwordNodeCard.locator('button[title="Edit node"]').click();
    await page.waitForSelector('h3:has-text("Edit PBS Node")', { timeout: 5000 });
    await delay(1000);
    
    // Check if password auth is selected
    const passwordRadioChecked = await page.locator('input[type="radio"][value="password"]').isChecked();
    if (passwordRadioChecked) {
      console.log('✅ Edit mode correctly shows password authentication selected');
    } else {
      console.log('❌ Edit mode incorrectly shows token authentication selected');
    }
    
    // Cancel edit
    await page.locator('button:has-text("Cancel")').click();
    await delay(1000);
    
    // TEST 5: Switch authentication type on existing node
    console.log('\n=== TEST 3: Switching Authentication Types ===\n');
    
    console.log('🔄 Converting password node to token auth...');
    await passwordNodeCard.locator('button[title="Edit node"]').click();
    await page.waitForSelector('h3:has-text("Edit PBS Node")', { timeout: 5000 });
    
    // Switch to token auth
    await page.locator('label:has-text("API Token")').click();
    await delay(500);
    
    // Fill token fields
    await page.fill('input[placeholder="user@realm!tokenname"]', TOKEN_AUTH.tokenName);
    await page.fill('input[placeholder="Enter token value"]', TOKEN_AUTH.tokenValue);
    
    // Update the node
    await page.locator('button:has-text("Update Node")').click();
    await delay(2000);
    
    // Verify it now shows "Token:"
    console.log('🔍 Verifying switched node shows token auth...');
    const switchedNodeCard = page.locator('.bg-white:has-text("PBS-Password")').first();
    const switchedAuthDisplay = await switchedNodeCard.locator('span.text-xs.px-2').first().textContent();
    console.log('   Display shows: ' + switchedAuthDisplay);
    
    if (switchedAuthDisplay.includes('Token:')) {
      console.log('✅ Successfully switched from password to token authentication');
    } else {
      console.log('❌ Failed to switch authentication type');
    }
    
    // Final summary
    console.log('\n' + '='.repeat(60));
    console.log('🎉 PBS Authentication Tests Complete!');
    console.log('='.repeat(60));
    
    // Take a screenshot of the final state
    await page.screenshot({ path: 'pbs-auth-test-final.png', fullPage: true });
    console.log('\n📸 Screenshot saved to pbs-auth-test-final.png');
    
  } catch (error) {
    console.error('❌ Test failed with error:', error);
    await page.screenshot({ path: 'pbs-auth-test-error.png', fullPage: true });
    console.log('📸 Error screenshot saved to pbs-auth-test-error.png');
  } finally {
    await delay(3000); // Keep browser open for 3 seconds to see final state
    await browser.close();
  }
}

// Run the test
testPBSAuthentication().catch(console.error);