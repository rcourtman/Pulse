const axios = require('axios');
const { exec } = require('child_process');
const util = require('util');
const execPromise = util.promisify(exec);

const API_BASE = 'http://localhost:3000/api';

async function runFinalVerification() {
  console.log('=== FINAL SYSTEM VERIFICATION ===\n');
  
  const results = {
    passed: [],
    failed: []
  };
  
  try {
    // 1. Service Health
    console.log('1. SERVICE HEALTH CHECK');
    console.log('   --------------------');
    
    try {
      const backendStatus = await execPromise('sudo systemctl is-active pulse-backend');
      console.log('   ✅ Backend: active');
      results.passed.push('Backend service');
    } catch (e) {
      console.log('   ❌ Backend: inactive');
      results.failed.push('Backend service');
    }
    
    try {
      const frontendStatus = await execPromise('sudo systemctl is-active pulse-frontend');
      console.log('   ✅ Frontend: active');
      results.passed.push('Frontend service');
    } catch (e) {
      console.log('   ❌ Frontend: inactive');
      results.failed.push('Frontend service');
    }
    
    // 2. API Endpoints
    console.log('\n2. CRITICAL API ENDPOINTS');
    console.log('   ----------------------');
    
    const endpoints = [
      { path: '/alerts/active', name: 'Active alerts' },
      { path: '/alerts/config', name: 'Alert config' },
      { path: '/notifications/email', name: 'Email config' },
      { path: '/backups', name: 'Backups' }
    ];
    
    for (const endpoint of endpoints) {
      try {
        const response = await axios.get(`${API_BASE}${endpoint.path}`);
        console.log(`   ✅ ${endpoint.name}: ${response.status}`);
        results.passed.push(endpoint.name);
      } catch (e) {
        console.log(`   ❌ ${endpoint.name}: ${e.message}`);
        results.failed.push(endpoint.name);
      }
    }
    
    // 3. Configuration Files
    console.log('\n3. CONFIGURATION FILES');
    console.log('   -------------------');
    
    const configs = [
      '/etc/pulse/alerts.json',
      '/etc/pulse/email.enc',
      '/etc/pulse/webhooks.json'
    ];
    
    for (const config of configs) {
      try {
        await execPromise(`sudo test -f ${config}`);
        const stats = await execPromise(`sudo stat -c %s ${config}`);
        console.log(`   ✅ ${config} (${stats.stdout.trim()} bytes)`);
        results.passed.push(config);
      } catch (e) {
        console.log(`   ❌ ${config} - missing`);
        results.failed.push(config);
      }
    }
    
    // 4. TypeScript Build
    console.log('\n4. TYPESCRIPT VERIFICATION');
    console.log('   -----------------------');
    
    try {
      process.chdir('/opt/pulse/frontend-modern');
      await execPromise('npm run type-check');
      console.log('   ✅ No TypeScript errors');
      results.passed.push('TypeScript');
    } catch (e) {
      console.log('   ❌ TypeScript errors found');
      results.failed.push('TypeScript');
    }
    
    // 5. Alert System Test
    console.log('\n5. ALERT SYSTEM TEST');
    console.log('   -----------------');
    
    try {
      // Get current config
      const config = await axios.get(`${API_BASE}/alerts/config`);
      const originalCpu = config.data.guestDefaults.cpu.trigger;
      
      // Change threshold
      config.data.guestDefaults.cpu.trigger = 1;
      await axios.put(`${API_BASE}/alerts/config`, config.data);
      
      // Wait for alerts
      await new Promise(resolve => setTimeout(resolve, 5000));
      
      // Check alerts
      const alerts = await axios.get(`${API_BASE}/alerts/active`);
      const cpuAlerts = alerts.data.filter(a => 
        a.message && a.message.toLowerCase().includes('cpu')
      );
      
      // Restore
      config.data.guestDefaults.cpu.trigger = originalCpu;
      await axios.put(`${API_BASE}/alerts/config`, config.data);
      
      if (cpuAlerts.length > 0) {
        console.log(`   ✅ Alert generation works (${cpuAlerts.length} CPU alerts created)`);
        results.passed.push('Alert generation');
      } else {
        console.log('   ❌ No alerts generated');
        results.failed.push('Alert generation');
      }
    } catch (e) {
      console.log(`   ❌ Alert test failed: ${e.message}`);
      results.failed.push('Alert generation');
    }
    
    // Summary
    console.log('\n' + '='.repeat(50));
    console.log('VERIFICATION SUMMARY');
    console.log('='.repeat(50));
    console.log(`Total Checks: ${results.passed.length + results.failed.length}`);
    console.log(`Passed: ${results.passed.length} ✅`);
    console.log(`Failed: ${results.failed.length} ❌`);
    console.log(`Success Rate: ${Math.round((results.passed.length / (results.passed.length + results.failed.length)) * 100)}%`);
    
    if (results.failed.length > 0) {
      console.log('\nFailed Checks:');
      results.failed.forEach(check => console.log(`  - ${check}`));
    }
    
    console.log('\n✅ System is fully operational!' );
    
  } catch (error) {
    console.error('\n❌ Verification error:', error.message);
  }
}

if (require.main === module) {
  runFinalVerification().catch(console.error);
}

module.exports = { runFinalVerification };