const axios = require('axios');
const { exec } = require('child_process');
const util = require('util');
const execPromise = util.promisify(exec);

const API_BASE = 'http://localhost:3000/api';

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

async function testComprehensiveSettings() {
  console.log('=== COMPREHENSIVE PULSE SETTINGS TEST ===\n');
  
  try {
    // 1. ALERT THRESHOLDS
    console.log('1. TESTING ALERT THRESHOLDS');
    console.log('   ------------------------');
    
    const alertConfig = await axios.get(`${API_BASE}/alerts/config`);
    logTest('Load alert config', !!alertConfig.data);
    
    // Test threshold changes
    const originalCpu = alertConfig.data.guestDefaults.cpu.trigger;
    alertConfig.data.guestDefaults.cpu.trigger = 88;
    const updateResp = await axios.put(`${API_BASE}/alerts/config`, alertConfig.data);
    logTest('Update CPU threshold', updateResp.status === 200);
    
    // Verify persistence
    const verifyConfig = await axios.get(`${API_BASE}/alerts/config`);
    logTest('CPU threshold persisted', verifyConfig.data.guestDefaults.cpu.trigger === 88);
    
    // Reset
    alertConfig.data.guestDefaults.cpu.trigger = originalCpu;
    await axios.put(`${API_BASE}/alerts/config`, alertConfig.data);
    
    // 2. EMAIL NOTIFICATIONS
    console.log('\n2. TESTING EMAIL NOTIFICATIONS');
    console.log('   ---------------------------');
    
    const emailConfig = await axios.get(`${API_BASE}/notifications/email`);
    logTest('Load email config', !!emailConfig.data);
    logTest('Email enabled', emailConfig.data.enabled === true);
    
    // 3. WEBHOOK NOTIFICATIONS
    console.log('\n3. TESTING WEBHOOK NOTIFICATIONS');
    console.log('   ------------------------------');
    
    const webhook = {
      name: 'Test Webhook',
      url: 'https://httpbin.org/post',
      method: 'POST',
      headers: {},
      template: '0',
      enabled: true
    };
    
    const webhookResp = await axios.post(`${API_BASE}/notifications/webhooks`, webhook);
    logTest('Create webhook', webhookResp.status === 200);
    
    if (webhookResp.data.id) {
      await axios.delete(`${API_BASE}/notifications/webhooks/${webhookResp.data.id}`);
      logTest('Delete webhook', true);
    }
    
    // 4. FILE ENCRYPTION
    console.log('\n4. TESTING FILE ENCRYPTION');
    console.log('   ------------------------');
    
    try {
      const { stdout: emailFile } = await execPromise('sudo file /etc/pulse/email.enc');
      logTest('email.enc encrypted', !emailFile.includes('JSON') && !emailFile.includes('text'));
    } catch (e) {
      logTest('File encryption check', false, e.message);
    }
    
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
  }
}

// Run the test
if (require.main === module) {
  testComprehensiveSettings().catch(console.error);
}

module.exports = { testComprehensiveSettings };