#!/usr/bin/env node

// Test to verify PBS form doesn't get contaminated with PVE data
// and that editing PBS nodes properly populates the form

const puppeteer = require('puppeteer');

async function test() {
  const browser = await puppeteer.launch({ 
    headless: 'new',
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });
  const page = await browser.newPage();
  
  console.log('Testing PBS form contamination fix...\n');
  
  try {
    // Navigate to Pulse
    await page.goto('http://localhost:7655', { waitUntil: 'networkidle0' });
    console.log('✓ Connected to Pulse');
    
    // Go to Settings - find the Settings nav item
    await page.waitForSelector('nav');
    const settingsLink = await page.$('a:has-text("Settings")') || await page.$('button:has-text("Settings")');
    if (settingsLink) {
      await settingsLink.click();
    } else {
      // Try clicking by text
      await page.evaluate(() => {
        const links = Array.from(document.querySelectorAll('a, button'));
        const settings = links.find(el => el.textContent?.includes('Settings'));
        if (settings) settings.click();
      });
    }
    await page.waitForTimeout(1000);
    console.log('✓ Navigated to Settings');
    
    // Test 1: Add a PVE node first
    console.log('\n--- Test 1: Add PVE node ---');
    await page.click('button:has-text("Add PVE Node")');
    await page.waitForSelector('h3:has-text("Add Proxmox VE Node")');
    
    // Fill PVE form
    await page.type('input[placeholder="pve.example.com or IP"]', 'pve-test.local');
    await page.type('input[placeholder="https://pve.example.com:8006"]', 'https://192.168.1.100:8006');
    
    // Close PVE modal
    await page.keyboard.press('Escape');
    await page.waitForTimeout(500);
    console.log('✓ Filled and closed PVE form');
    
    // Test 2: Now add PBS node - should NOT have PVE data
    console.log('\n--- Test 2: Add PBS after PVE - check for contamination ---');
    await page.click('button:has-text("Add PBS Node")');
    await page.waitForSelector('h3:has-text("Add Proxmox Backup Server Node")');
    
    // Check that PBS form fields are empty (not contaminated with PVE data)
    const pbsName = await page.$eval('input[placeholder="pbs.example.com or IP"]', el => el.value);
    const pbsHost = await page.$eval('input[placeholder="https://pbs.example.com:8007"]', el => el.value);
    
    if (pbsName === '' && pbsHost === '') {
      console.log('✓ PBS form is clean - no PVE contamination');
    } else {
      console.log('✗ FAIL: PBS form contaminated with data:', { pbsName, pbsHost });
      throw new Error('PBS form contaminated with PVE data');
    }
    
    // Fill PBS form for next test
    await page.type('input[placeholder="pbs.example.com or IP"]', 'pbs-test.local');
    await page.type('input[placeholder="https://pbs.example.com:8007"]', 'https://192.168.1.200:8007');
    await page.type('input[placeholder="root@pam!tokenname"]', 'root@pam!test-token');
    await page.type('input[placeholder*="xxxx"]', 'test-token-value-12345');
    
    // Save PBS node
    await page.click('button:has-text("Add Node")');
    await page.waitForTimeout(1000);
    console.log('✓ PBS node added');
    
    // Test 3: Edit PBS node - should populate with PBS data
    console.log('\n--- Test 3: Edit PBS node - check data population ---');
    
    // Find and click edit button for PBS node
    const pbsCard = await page.$('div:has(h4:has-text("pbs-test.local"))');
    if (pbsCard) {
      const editButton = await pbsCard.$('button[title*="dit"]');
      if (editButton) {
        await editButton.click();
        await page.waitForSelector('h3:has-text("Edit Proxmox Backup Server Node")');
        
        // Check that form is populated with PBS data
        const editName = await page.$eval('input[placeholder="pbs.example.com or IP"]', el => el.value);
        const editHost = await page.$eval('input[placeholder="https://pbs.example.com:8007"]', el => el.value);
        const editToken = await page.$eval('input[placeholder="root@pam!tokenname"]', el => el.value);
        
        if (editName === 'pbs-test.local' && 
            editHost === 'https://192.168.1.200:8007' && 
            editToken === 'root@pam!test-token') {
          console.log('✓ PBS edit form correctly populated with PBS data');
        } else {
          console.log('✗ FAIL: PBS edit form not populated correctly:', { editName, editHost, editToken });
          throw new Error('PBS edit form not populated correctly');
        }
      }
    }
    
    // Close modal
    await page.keyboard.press('Escape');
    await page.waitForTimeout(500);
    
    // Test 4: Edit PVE after PBS - ensure no cross-contamination
    console.log('\n--- Test 4: Edit PVE after PBS - check for contamination ---');
    
    // First need to add a real PVE node to edit
    await page.click('button:has-text("Add PVE Node")');
    await page.waitForSelector('h3:has-text("Add Proxmox VE Node")');
    await page.type('input[placeholder="pve.example.com or IP"]', 'pve-real.local');
    await page.type('input[placeholder="https://pve.example.com:8006"]', 'https://192.168.1.50:8006');
    await page.type('input[placeholder="root@pam!tokenname"]', 'root@pam!pve-token');
    await page.type('input[placeholder*="xxxx"]', 'pve-token-value-54321');
    await page.click('button:has-text("Add Node")');
    await page.waitForTimeout(1000);
    
    // Now edit the PVE node
    const pveCard = await page.$('div:has(h4:has-text("pve-real.local"))');
    if (pveCard) {
      const editButton = await pveCard.$('button[title*="dit"]');
      if (editButton) {
        await editButton.click();
        await page.waitForSelector('h3:has-text("Edit Proxmox VE Node")');
        
        // Check that form is populated with PVE data, not PBS data
        const pveEditName = await page.$eval('input[placeholder="pve.example.com or IP"]', el => el.value);
        const pveEditHost = await page.$eval('input[placeholder="https://pve.example.com:8006"]', el => el.value);
        
        if (pveEditName === 'pve-real.local' && pveEditHost === 'https://192.168.1.50:8006') {
          console.log('✓ PVE edit form correctly populated with PVE data (no PBS contamination)');
        } else {
          console.log('✗ FAIL: PVE edit form contaminated or incorrect:', { pveEditName, pveEditHost });
          throw new Error('PVE edit form contaminated or incorrect');
        }
      }
    }
    
    console.log('\n========================================');
    console.log('✓ ALL TESTS PASSED - PBS form fix verified!');
    console.log('========================================\n');
    
  } catch (error) {
    console.error('\n✗ TEST FAILED:', error.message);
    await browser.close();
    process.exit(1);
  }
  
  await browser.close();
}

test().catch(console.error);