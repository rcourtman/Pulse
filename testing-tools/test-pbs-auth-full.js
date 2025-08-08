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
    console.log('🚀 PBS Authentication Test\n');
    console.log('=' .repeat(60));
    
    // Navigate to Pulse
    console.log('\n📍 Navigating to Pulse...');
    await page.goto(BASE_URL);
    await delay(2000);
    
    // Go to Settings
    console.log('⚙️  Opening Settings...');
    await page.locator('text=Settings').first().click();
    await delay(1500);
    
    // Click on PBS Nodes tab
    console.log('📂 Opening PBS Nodes tab...');
    await page.locator('button:text("PBS Nodes")').click();
    await delay(1000);
    
    // Clean up existing PBS nodes
    console.log('🧹 Cleaning up existing PBS nodes...');
    const existingCards = await page.locator('.bg-white').filter({ hasText: 'PBS' }).all();
    for (const card of existingCards) {
      const deleteBtn = card.locator('button[title="Delete node"]');
      if (await deleteBtn.isVisible()) {
        await deleteBtn.click();
        await delay(500);
        const confirmBtn = page.locator('button').filter({ hasText: 'Delete' }).last();
        if (await confirmBtn.isVisible()) {
          await confirmBtn.click();
          await delay(1000);
        }
      }
    }
    
    console.log('\n' + '='.repeat(60));
    console.log('TEST 1: Token-based Authentication');
    console.log('='.repeat(60) + '\n');
    
    // Add PBS node with token auth
    console.log('➕ Adding PBS node with token authentication...');
    
    // Look for the Add button - it might be "Add Node" when PBS tab is selected
    let addButton = page.locator('button:has-text("Add PBS Node")');
    if (!await addButton.isVisible({ timeout: 1000 })) {
      addButton = page.locator('button:has-text("Add Node")').first();
    }
    if (!await addButton.isVisible({ timeout: 1000 })) {
      addButton = page.locator('button').filter({ hasText: /Add/i }).first();
    }
    
    await addButton.click();
    await delay(2000);
    
    // Wait for modal to appear
    await page.waitForSelector('text=/Add PBS|New PBS/i', { timeout: 5000 });
    
    // Fill token auth form - use more flexible selectors
    console.log('📝 Filling token authentication form...');
    const nameInput = page.locator('input').filter({ hasPlaceholder: /name/i }).first();
    await nameInput.fill('PBS-Token-Test');
    const hostInput = page.locator('input').filter({ hasPlaceholder: /pbs|host|server/i }).nth(1);
    await hostInput.fill(PBS_HOST);
    
    // Select API Token auth
    await page.locator('label:has-text("API Token")').click();
    await delay(500);
    
    const tokenNameInput = page.locator('input').filter({ hasPlaceholder: /token.*name|tokenname/i }).first();
    await tokenNameInput.fill(TOKEN_AUTH.tokenName);
    
    const tokenValueInput = page.locator('input').filter({ hasPlaceholder: /token.*value|secret/i }).first();
    await tokenValueInput.fill(TOKEN_AUTH.tokenValue);
    
    // Test connection
    console.log('🔌 Testing token connection...');
    await page.locator('button:has-text("Test Connection")').click();
    await delay(3000);
    
    // Check result - look for success/error in the modal, not the instructions
    const modalResult = page.locator('div[role="dialog"], .fixed').last();
    const tokenSuccess = await modalResult.locator('.text-green-600, .text-green-500').first().isVisible({ timeout: 1000 }).catch(() => false);
    if (tokenSuccess) {
      const msg = await modalResult.locator('.text-green-600, .text-green-500').first().textContent();
      console.log('✅ Token test: ' + msg);
    } else {
      const error = await modalResult.locator('.text-red-600, .text-red-500').first();
      if (await error.isVisible({ timeout: 1000 })) {
        console.log('❌ Token test failed: ' + await error.textContent());
      } else {
        console.log('⚠️  No clear test result shown');
      }
    }
    
    // Save node
    await page.locator('button:has-text("Add Node")').click();
    await delay(2000);
    
    // Verify display shows "Token:"
    console.log('\n🔍 Verifying token authentication display...');
    const tokenCard = page.locator('.bg-white').filter({ hasText: 'PBS-Token-Test' }).first();
    const tokenDisplay = await tokenCard.locator('span.text-xs').first().textContent();
    
    if (tokenDisplay.includes('Token:')) {
      console.log('✅ Correctly shows: ' + tokenDisplay);
    } else {
      console.log('❌ Incorrectly shows: ' + tokenDisplay);
    }
    
    // Test edit mode
    console.log('\n📝 Testing edit mode preserves token auth...');
    await tokenCard.locator('button[title="Edit node"]').click();
    await delay(1000);
    
    const tokenRadioChecked = await page.locator('input[value="token"]').isChecked();
    if (tokenRadioChecked) {
      console.log('✅ Edit mode correctly shows token auth selected');
    } else {
      console.log('❌ Edit mode incorrectly shows password auth selected');
    }
    
    await page.locator('button:has-text("Cancel")').click();
    await delay(1000);
    
    console.log('\n' + '='.repeat(60));
    console.log('TEST 2: Password-based Authentication');
    console.log('='.repeat(60) + '\n');
    
    // Add PBS node with password auth
    console.log('➕ Adding PBS node with password authentication...');
    let addButton2 = page.locator('button:has-text("Add PBS Node")');
    if (!await addButton2.isVisible({ timeout: 1000 })) {
      addButton2 = page.locator('button:has-text("Add Node")').first();
    }
    if (!await addButton2.isVisible({ timeout: 1000 })) {
      addButton2 = page.locator('button').filter({ hasText: /Add/i }).first();
    }
    await addButton2.click();
    await delay(2000);
    
    console.log('📝 Filling password authentication form...');
    const nameInput2 = page.locator('input').filter({ hasPlaceholder: /name/i }).first();
    await nameInput2.fill('PBS-Password-Test');
    
    const hostInput2 = page.locator('input').filter({ hasPlaceholder: /pbs|host|server/i }).nth(1);
    await hostInput2.fill(PBS_HOST);
    
    // Select password auth (should be default)
    await page.locator('label:has-text("Username/Password")').click();
    await delay(500);
    
    const userInput = page.locator('input').filter({ hasPlaceholder: /user.*realm|username/i }).first();
    await userInput.fill(PASSWORD_AUTH.user);
    
    const passInput = page.locator('input[type="password"]').first();
    await passInput.fill(PASSWORD_AUTH.password);
    
    // Test connection
    console.log('🔌 Testing password connection...');
    await page.locator('button:has-text("Test Connection")').click();
    await delay(3000);
    
    // Check result - look for success/error in the modal, not the instructions  
    const modalResult2 = page.locator('div[role="dialog"], .fixed').last();
    const passSuccess = await modalResult2.locator('.text-green-600, .text-green-500').first().isVisible({ timeout: 1000 }).catch(() => false);
    if (passSuccess) {
      const msg = await modalResult2.locator('.text-green-600, .text-green-500').first().textContent();
      console.log('✅ Password test: ' + msg);
    } else {
      const error = await modalResult2.locator('.text-red-600, .text-red-500').first();
      if (await error.isVisible({ timeout: 1000 })) {
        console.log('❌ Password test failed: ' + await error.textContent());
      } else {
        console.log('⚠️  No clear test result shown');
      }
    }
    
    // Save node
    await page.locator('button:has-text("Add Node")').click();
    await delay(2000);
    
    // Verify display shows "User:"
    console.log('\n🔍 Verifying password authentication display...');
    const passCard = page.locator('.bg-white').filter({ hasText: 'PBS-Password-Test' }).first();
    const passDisplay = await passCard.locator('span.text-xs').first().textContent();
    
    if (passDisplay.includes('User:')) {
      console.log('✅ Correctly shows: ' + passDisplay);
    } else {
      console.log('❌ Incorrectly shows: ' + passDisplay);
    }
    
    // Test edit mode
    console.log('\n📝 Testing edit mode preserves password auth...');
    await passCard.locator('button[title="Edit node"]').click();
    await delay(1000);
    
    const passRadioChecked = await page.locator('input[value="password"]').isChecked();
    if (passRadioChecked) {
      console.log('✅ Edit mode correctly shows password auth selected');
    } else {
      console.log('❌ Edit mode incorrectly shows token auth selected');
    }
    
    await page.locator('button:has-text("Cancel")').click();
    await delay(1000);
    
    console.log('\n' + '='.repeat(60));
    console.log('TEST 3: Switching Authentication Types');
    console.log('='.repeat(60) + '\n');
    
    // Switch password node to token
    console.log('🔄 Converting password node to token auth...');
    await passCard.locator('button[title="Edit node"]').click();
    await delay(1000);
    
    await page.locator('label:has-text("API Token")').click();
    await delay(500);
    
    const tokenNameInput2 = page.locator('input').filter({ hasPlaceholder: /token.*name|tokenname/i }).first();
    await tokenNameInput2.fill(TOKEN_AUTH.tokenName);
    
    const tokenValueInput2 = page.locator('input').filter({ hasPlaceholder: /token.*value|secret/i }).first();
    await tokenValueInput2.fill(TOKEN_AUTH.tokenValue);
    
    await page.locator('button:has-text("Update Node")').click();
    await delay(2000);
    
    // Verify it now shows "Token:"
    const switchedDisplay = await passCard.locator('span.text-xs').first().textContent();
    if (switchedDisplay.includes('Token:')) {
      console.log('✅ Successfully switched to token auth: ' + switchedDisplay);
    } else {
      console.log('❌ Failed to switch auth type: ' + switchedDisplay);
    }
    
    // Final screenshot
    await page.screenshot({ path: 'pbs-auth-test-complete.png', fullPage: true });
    
    console.log('\n' + '='.repeat(60));
    console.log('✨ PBS Authentication Tests Complete!');
    console.log('='.repeat(60));
    console.log('\n📸 Final screenshot: pbs-auth-test-complete.png');
    
  } catch (error) {
    console.error('\n❌ Test failed:', error.message);
    await page.screenshot({ path: 'pbs-auth-test-error.png' });
    console.log('📸 Error screenshot: pbs-auth-test-error.png');
  } finally {
    await browser.close();
  }
}

testPBSAuthentication().catch(console.error);