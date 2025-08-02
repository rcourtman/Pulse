const axios = require('axios');

const API_BASE = 'http://localhost:3000/api';

async function testAPIEndpoints() {
  console.log('Testing Pulse API Endpoints...\n');
  
  const results = {
    passed: 0,
    failed: 0,
    endpoints: []
  };
  
  async function testEndpoint(method, path, data = null, description = '') {
    try {
      const config = {
        method,
        url: `${API_BASE}${path}`,
        data,
        validateStatus: () => true // Don't throw on any status
      };
      
      const response = await axios(config);
      const passed = response.status >= 200 && response.status < 400;
      
      console.log(`${passed ? '✅' : '❌'} ${method} ${path} - ${response.status}${description ? ' - ' + description : ''}`);
      
      results.endpoints.push({ method, path, status: response.status, passed });
      if (passed) results.passed++;
      else results.failed++;
      
      return response;
    } catch (error) {
      console.log(`❌ ${method} ${path} - ${error.message}`);
      results.endpoints.push({ method, path, error: error.message, passed: false });
      results.failed++;
      return null;
    }
  }
  
  // Test Alert Endpoints
  console.log('1. ALERT ENDPOINTS');
  console.log('   ---------------');
  await testEndpoint('GET', '/alerts/active', null, 'Get active alerts');
  await testEndpoint('GET', '/alerts/history', null, 'Get alert history');
  await testEndpoint('GET', '/alerts/config', null, 'Get alert configuration');
  
  // Test Notification Endpoints
  console.log('\n2. NOTIFICATION ENDPOINTS');
  console.log('   ----------------------');
  await testEndpoint('GET', '/notifications/email', null, 'Get email config');
  await testEndpoint('GET', '/notifications/webhooks', null, 'Get webhooks');
  await testEndpoint('GET', '/notifications/email-providers', null, 'Get email providers');
  await testEndpoint('GET', '/notifications/webhook-templates', null, 'Get webhook templates');
  
  // Test Data Endpoints
  console.log('\n3. DATA ENDPOINTS');
  console.log('   ---------------');
  await testEndpoint('GET', '/data', null, 'Get monitoring data');
  await testEndpoint('GET', '/data/history', null, 'Get historical data');
  await testEndpoint('GET', '/chart/guest/101', null, 'Get guest chart data');
  
  // Test Storage Endpoints
  console.log('\n4. STORAGE ENDPOINTS');
  console.log('   ------------------');
  await testEndpoint('GET', '/storage', null, 'Get storage info');
  await testEndpoint('GET', '/storage/history', null, 'Get storage history');
  
  // Test Backup Endpoints
  console.log('\n5. BACKUP ENDPOINTS');
  console.log('   -----------------');
  await testEndpoint('GET', '/backups', null, 'Get backups list');
  
  // Summary
  console.log('\n' + '='.repeat(50));
  console.log('API TEST SUMMARY');
  console.log('='.repeat(50));
  console.log(`Total Endpoints: ${results.passed + results.failed}`);
  console.log(`Passed: ${results.passed} ✅`);
  console.log(`Failed: ${results.failed} ❌`);
  console.log(`Success Rate: ${Math.round((results.passed / (results.passed + results.failed)) * 100)}%`);
  
  return results;
}

// Run the test
if (require.main === module) {
  testAPIEndpoints().catch(console.error);
}

module.exports = { testAPIEndpoints };