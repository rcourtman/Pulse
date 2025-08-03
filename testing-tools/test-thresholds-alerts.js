const axios = require('axios');
const { chromium } = require('playwright');

const API_BASE = 'http://localhost:3000/api';
const FRONTEND_URL = 'http://localhost:7655';

// Test results tracking
const results = {
  passed: 0,
  failed: 0,
  tests: []
};

function logTest(name, passed, details = '') {
  const status = passed ? '✅ PASS' : '❌ FAIL';
  console.log(`   ${status}: ${name}${details ? ' - ' + details : ''}`);
  results.tests.push({ name, passed, details });
  if (passed) results.passed++;
  else results.failed++;
}

async function waitForAlerts(expectedCount, maxWait = 30000) {
  const startTime = Date.now();
  while (Date.now() - startTime < maxWait) {
    try {
      const response = await axios.get(`${API_BASE}/alerts/active`);
      if (response.data.length >= expectedCount) {
        return response.data;
      }
    } catch (e) {
      // Continue waiting
    }
    await new Promise(resolve => setTimeout(resolve, 1000));
  }
  return [];
}

async function testThresholdsAndAlerts() {
  console.log('=== COMPREHENSIVE THRESHOLD AND ALERT TESTING ===\n');
  
  const browser = await chromium.launch({ headless: true });
  
  try {
    // 1. GET CURRENT CONFIGURATION
    console.log('1. LOADING CURRENT CONFIGURATION');
    console.log('   ------------------------------');
    
    const configResponse = await axios.get(`${API_BASE}/alerts/config`);
    const originalConfig = JSON.parse(JSON.stringify(configResponse.data));
    logTest('Load alert configuration', !!originalConfig);
    
    // Store original thresholds
    const originalThresholds = {
      cpu: originalConfig.guestDefaults.cpu.trigger,
      memory: originalConfig.guestDefaults.memory.trigger,
      disk: originalConfig.guestDefaults.disk.trigger,
      diskRead: originalConfig.guestDefaults.diskRead.trigger,
      diskWrite: originalConfig.guestDefaults.diskWrite.trigger,
      networkIn: originalConfig.guestDefaults.networkIn.trigger,
      networkOut: originalConfig.guestDefaults.networkOut.trigger
    };
    
    console.log('   Original thresholds:', JSON.stringify(originalThresholds, null, 2));
    
    // 2. CLEAR ALL EXISTING ALERTS
    console.log('\n2. CLEARING EXISTING ALERTS');
    console.log('   ------------------------');
    
    const existingAlerts = await axios.get(`${API_BASE}/alerts/active`);
    console.log(`   Found ${existingAlerts.data.length} existing alerts`);
    
    // Clear all alerts (acknowledge them since some may be persistent)
    for (const alert of existingAlerts.data) {
      try {
        await axios.post(`${API_BASE}/alerts/${alert.id}/acknowledge`);
        console.log(`   Acknowledged alert: ${alert.resourceName} - ${alert.message}`);
      } catch (e) {
        console.log(`   Warning: Could not acknowledge alert ${alert.id}: ${e.message}`);
      }
    }
    
    // Wait for alerts to clear
    await new Promise(resolve => setTimeout(resolve, 2000));
    const clearedCheck = await axios.get(`${API_BASE}/alerts/active`);
    logTest('Clear all existing alerts', clearedCheck.data.length === 0, `${clearedCheck.data.length} alerts remaining`);
    
    // 3. TEST THROUGH UI
    console.log('\n3. TESTING THRESHOLD CHANGES THROUGH UI');
    console.log('   ------------------------------------');
    
    const context = await browser.newContext();
    const page = await context.newPage();
    
    // Navigate to alerts page
    await page.goto(`${FRONTEND_URL}/alerts`);
    await page.waitForTimeout(3000);
    
    // Test different threshold types
    const thresholdTests = [
      { type: 'CPU', slider: 'cpu', newValue: 1, expectedAlerts: 5 },
      { type: 'Memory', slider: 'memory', newValue: 5, expectedAlerts: 8 },
      { type: 'Disk', slider: 'disk', newValue: 10, expectedAlerts: 3 }
    ];
    
    for (const test of thresholdTests) {
      console.log(`\n   Testing ${test.type} threshold:`);
      
      // Find and adjust the slider
      const sliderSelector = `input[type="range"][name="${test.slider}"]`;
      await page.waitForSelector(sliderSelector);
      
      // Set new value
      await page.fill(sliderSelector, test.newValue.toString());
      await page.dispatchEvent(sliderSelector, 'input');
      await page.waitForTimeout(500);
      
      // Save changes
      const saveButton = await page.$('button:has-text("Save Changes")');
      if (saveButton) {
        await saveButton.click();
        await page.waitForTimeout(1000);
        
        // Check for success message
        const hasSuccess = await page.evaluate(() => {
          const toasts = Array.from(document.querySelectorAll('.toast-success'));
          return toasts.some(t => t.textContent.includes('saved successfully'));
        });
        
        logTest(`Set ${test.type} threshold to ${test.newValue}%`, hasSuccess);
        
        // Wait for alerts to be generated
        console.log(`   Waiting for ${test.type} alerts to be generated...`);
        await page.waitForTimeout(5000); // Give time for alerts to trigger
        
        // Check alerts via API
        const alerts = await axios.get(`${API_BASE}/alerts/active`);
        const typeAlerts = alerts.data.filter(a => {
          // Check both metric field and message content for the metric type
          const metric = a.metric || '';
          const message = a.message || '';
          return metric.toLowerCase() === test.slider.toLowerCase() ||
                 message.toLowerCase().includes(test.slider.toLowerCase());
        });
        
        console.log(`   Found ${typeAlerts.length} ${test.type} alerts`);
        logTest(
          `${test.type} alerts generated`, 
          typeAlerts.length > 0,
          `${typeAlerts.length} alerts found`
        );
        
        // List affected resources
        if (typeAlerts.length > 0) {
          console.log(`   Affected resources:`);
          typeAlerts.slice(0, 5).forEach(alert => {
            console.log(`     - ${alert.resourceName}: ${alert.value}% (threshold: ${test.newValue}%)`);
          });
          if (typeAlerts.length > 5) {
            console.log(`     ... and ${typeAlerts.length - 5} more`);
          }
        }
      }
    }
    
    // 4. TEST ALERT ACKNOWLEDGEMENT
    console.log('\n4. TESTING ALERT ACKNOWLEDGEMENT');
    console.log('   ------------------------------');
    
    const currentAlerts = await axios.get(`${API_BASE}/alerts/active`);
    if (currentAlerts.data.length > 0) {
      const testAlert = currentAlerts.data[0];
      
      try {
        await axios.post(`${API_BASE}/alerts/${testAlert.id}/acknowledge`);
        
        // Check if acknowledged
        const updatedAlerts = await axios.get(`${API_BASE}/alerts/active`);
        const acknowledgedAlert = updatedAlerts.data.find(a => a.id === testAlert.id);
        
        logTest(
          'Acknowledge alert',
          acknowledgedAlert && acknowledgedAlert.acknowledged === true,
          `Alert ${testAlert.resourceName} - ${testAlert.metric}`
        );
      } catch (e) {
        logTest('Acknowledge alert', false, e.message);
      }
    }
    
    // 5. RESTORE ORIGINAL THRESHOLDS
    console.log('\n5. RESTORING ORIGINAL THRESHOLDS');
    console.log('   ------------------------------');
    
    // Restore each threshold
    for (const [metric, value] of Object.entries(originalThresholds)) {
      const sliderSelector = `input[type="range"][name="${metric}"]`;
      await page.waitForSelector(sliderSelector);
      await page.fill(sliderSelector, value.toString());
      await page.dispatchEvent(sliderSelector, 'input');
    }
    
    // Save restored values
    const finalSaveButton = await page.$('button:has-text("Save Changes")');
    if (finalSaveButton) {
      await finalSaveButton.click();
      await page.waitForTimeout(1000);
    }
    
    // Verify restoration
    const restoredConfig = await axios.get(`${API_BASE}/alerts/config`);
    const allRestored = Object.entries(originalThresholds).every(([metric, value]) => {
      const restored = restoredConfig.data.guestDefaults[metric].trigger === value;
      return restored;
    });
    
    logTest('Restore original thresholds', allRestored);
    
    // 6. WAIT FOR ALERTS TO CLEAR
    console.log('\n6. VERIFYING ALERTS CLEAR');
    console.log('   -----------------------');
    
    console.log('   Waiting for alerts to clear naturally...');
    await page.waitForTimeout(10000); // Wait for next check cycle
    
    const finalAlerts = await axios.get(`${API_BASE}/alerts/active`);
    const remainingAlerts = finalAlerts.data.filter(a => !a.acknowledged);
    
    console.log(`   ${remainingAlerts.length} active alerts remaining`);
    logTest(
      'Alerts clear after threshold restoration',
      remainingAlerts.length < currentAlerts.data.length,
      `${currentAlerts.data.length} → ${remainingAlerts.length} alerts`
    );
    
    // Take screenshot of final state
    await page.screenshot({ path: 'alerts-test-final.png', fullPage: true });
    
    await context.close();
    
    // SUMMARY
    console.log('\n' + '='.repeat(50));
    console.log('TEST SUMMARY');
    console.log('='.repeat(50));
    console.log(`Total Tests: ${results.passed + results.failed}`);
    console.log(`Passed: ${results.passed} ✅`);
    console.log(`Failed: ${results.failed} ❌`);
    console.log(`Success Rate: ${Math.round((results.passed / (results.passed + results.failed)) * 100)}%`);
    
    if (results.failed > 0) {
      console.log('\nFailed Tests:');
      results.tests.filter(t => !t.passed).forEach(t => {
        console.log(`  - ${t.name}: ${t.details}`);
      });
    }
    
  } catch (error) {
    console.error('\n❌ Test suite error:', error.message);
  } finally {
    await browser.close();
  }
}

// Run the test
if (require.main === module) {
  testThresholdsAndAlerts().catch(console.error);
}

module.exports = { testThresholdsAndAlerts };