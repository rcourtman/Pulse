const { chromium } = require('playwright');

const BASE_URL = 'http://192.168.0.123:7655';

async function verifyPBSDisplay() {
  console.log('\n' + '='.repeat(60));
  console.log('PBS Authentication Display Verification');
  console.log('='.repeat(60) + '\n');
  console.log('This test assumes you have already added PBS nodes manually.\n');
  
  const browser = await chromium.launch({ 
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });
  
  const page = await browser.newContext({ ignoreHTTPSErrors: true })
    .then(ctx => ctx.newPage());
  
  try {
    // Navigate to Pulse
    console.log('📍 Navigating to Pulse...');
    await page.goto(BASE_URL);
    await page.waitForTimeout(2000);
    
    // Go to Settings
    console.log('⚙️  Opening Settings...');
    await page.locator('text=Settings').first().click();
    await page.waitForTimeout(1500);
    
    // Click on PBS Nodes tab
    console.log('📂 Opening PBS Nodes tab...\n');
    await page.locator('button:text("PBS Nodes")').click();
    await page.waitForTimeout(1000);
    
    // Take screenshot
    await page.screenshot({ path: 'pbs-display-check.png', fullPage: true });
    console.log('📸 Screenshot saved to pbs-display-check.png\n');
    
    // Find all PBS node cards
    const pbsCards = await page.locator('.bg-white, .dark\\:bg-gray-800').all();
    
    console.log(`Found ${pbsCards.length} node cards\n`);
    
    for (let i = 0; i < pbsCards.length; i++) {
      const card = pbsCards[i];
      
      // Try to get the node name
      const nameElement = card.locator('h4, .font-medium').first();
      const nodeName = await nameElement.textContent().catch(() => 'Unknown');
      
      // Look for auth display (User: or Token:)
      const authSpan = card.locator('span.text-xs').first();
      const authDisplay = await authSpan.textContent().catch(() => 'Not found');
      
      console.log(`Node ${i + 1}: ${nodeName}`);
      console.log(`  Auth display: ${authDisplay}`);
      
      // Check edit mode
      const editButton = card.locator('button[title="Edit node"]');
      if (await editButton.isVisible()) {
        console.log('  Testing edit mode...');
        await editButton.click();
        await page.waitForTimeout(1000);
        
        // Check which auth type is selected
        const tokenRadioChecked = await page.locator('input[value="token"]').isChecked();
        const passRadioChecked = await page.locator('input[value="password"]').isChecked();
        
        if (tokenRadioChecked) {
          console.log('  ✅ Edit mode shows: Token authentication');
          if (authDisplay.includes('Token:')) {
            console.log('  ✅ Display matches auth type');
          } else {
            console.log('  ❌ Display does NOT match auth type (shows ' + authDisplay + ')');
          }
        } else if (passRadioChecked) {
          console.log('  ✅ Edit mode shows: Password authentication');
          if (authDisplay.includes('User:')) {
            console.log('  ✅ Display matches auth type');
          } else {
            console.log('  ❌ Display does NOT match auth type (shows ' + authDisplay + ')');
          }
        } else {
          console.log('  ⚠️  Cannot determine auth type in edit mode');
        }
        
        // Cancel edit
        await page.locator('button:has-text("Cancel")').click();
        await page.waitForTimeout(1000);
      }
      
      console.log('');
    }
    
    console.log('='.repeat(60));
    console.log('Verification Complete');
    console.log('='.repeat(60));
    
  } catch (error) {
    console.error('Error:', error.message);
    await page.screenshot({ path: 'pbs-display-error.png' });
  } finally {
    await browser.close();
  }
}

verifyPBSDisplay().catch(console.error);