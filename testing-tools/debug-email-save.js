const { chromium } = require('playwright');

const FRONTEND_URL = 'http://192.168.0.123:7655';

async function debugEmailSave() {
  console.log('=== DEBUGGING EMAIL SAVE PROCESS ===\n');
  
  const browser = await chromium.launch({ 
    headless: false,  // Show browser
    slowMo: 500       // Slow down actions
  });
  
  try {
    const context = await browser.newContext();
    const page = await context.newPage();
    
    // Monitor console logs
    page.on('console', msg => {
      if (msg.text().includes('email') || msg.text().includes('recipient')) {
        console.log('[Browser]:', msg.text());
      }
    });
    
    // Navigate to alerts page
    console.log('1. Navigating to alerts page...');
    await page.goto(`${FRONTEND_URL}/alerts`);
    await page.waitForTimeout(2000);
    
    // Click on Destinations tab
    console.log('2. Clicking Destinations tab...');
    await page.click('button:has-text("Destinations")');
    await page.waitForTimeout(1000);
    
    // Check current recipients
    console.log('3. Checking recipients field...');
    const recipientsField = await page.locator('textarea').filter({ hasText: /leave empty|recipients/i });
    const currentRecipients = await recipientsField.inputValue();
    console.log('   Current recipients:', currentRecipients || '(empty)');
    
    // Add a test recipient
    console.log('4. Adding test recipient...');
    await recipientsField.fill('test@example.com');
    await page.waitForTimeout(500);
    
    // Save changes
    console.log('5. Clicking Save Changes...');
    await page.click('button:has-text("Save Changes")');
    await page.waitForTimeout(2000);
    
    // Navigate away and back
    console.log('6. Navigating to Overview tab...');
    await page.click('button:has-text("Overview")');
    await page.waitForTimeout(1000);
    
    console.log('7. Navigating back to Destinations...');
    await page.click('button:has-text("Destinations")');
    await page.waitForTimeout(1000);
    
    // Check if recipients persisted
    console.log('8. Checking if recipients persisted...');
    const recipientsAfter = await recipientsField.inputValue();
    console.log('   Recipients after save:', recipientsAfter || '(empty)');
    
    if (recipientsAfter.includes('test@example.com')) {
      console.log('   ✅ Recipients persisted correctly!');
    } else {
      console.log('   ❌ Recipients were not saved!');
    }
    
    console.log('\nPress Ctrl+C to close the browser...');
    await page.waitForTimeout(60000); // Keep browser open
    
  } catch (error) {
    console.error('Error:', error.message);
  } finally {
    await browser.close();
  }
}

if (require.main === module) {
  debugEmailSave().catch(console.error);
}

module.exports = { debugEmailSave };