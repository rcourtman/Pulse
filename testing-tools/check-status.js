const axios = require('axios');
const { exec } = require('child_process');
const util = require('util');
const execPromise = util.promisify(exec);

async function checkSystemStatus() {
  console.log('=== PULSE SYSTEM STATUS CHECK ===\n');
  
  // 1. Check Services
  console.log('1. SERVICE STATUS');
  console.log('   --------------');
  
  try {
    const { stdout: backendStatus } = await execPromise('sudo systemctl is-active pulse-backend');
    console.log(`   ✅ Backend: ${backendStatus.trim()}`);
  } catch (e) {
    console.log('   ❌ Backend: inactive');
  }
  
  try {
    const { stdout: frontendStatus } = await execPromise('sudo systemctl is-active pulse-frontend');
    console.log(`   ✅ Frontend: ${frontendStatus.trim()}`);
  } catch (e) {
    console.log('   ❌ Frontend: inactive');
  }
  
  // 2. Check API Health
  console.log('\n2. API HEALTH');
  console.log('   -----------');
  
  try {
    const response = await axios.get('http://localhost:3000/api/alerts/active');
    console.log(`   ✅ API responding: ${response.data.length} active alerts`);
  } catch (e) {
    console.log('   ❌ API not responding:', e.message);
  }
  
  // 3. Check Configuration Files
  console.log('\n3. CONFIGURATION FILES');
  console.log('   --------------------');
  
  const configFiles = [
    '/etc/pulse/alerts.json',
    '/etc/pulse/email.enc',
    '/etc/pulse/webhooks.json',
    '/opt/pulse/CLAUDE.md'
  ];
  
  for (const file of configFiles) {
    try {
      await execPromise(`sudo test -f ${file}`);
      const { stdout: size } = await execPromise(`sudo ls -lh ${file} | awk '{print $5}'`);
      console.log(`   ✅ ${file} (${size.trim()})`);
    } catch (e) {
      console.log(`   ❌ ${file} - missing`);
    }
  }
  
  // 4. Check Recent Logs
  console.log('\n4. RECENT LOG ENTRIES');
  console.log('   -------------------');
  
  try {
    const { stdout: logs } = await execPromise('sudo tail -5 /opt/pulse/pulse.log | grep -E "INFO|WARN|ERROR"');
    console.log(logs || '   No recent log entries');
  } catch (e) {
    console.log('   Unable to read logs');
  }
  
  // 5. Check Active Alerts
  console.log('\n5. ACTIVE ALERTS');
  console.log('   --------------');
  
  try {
    const alerts = await axios.get('http://localhost:3000/api/alerts/active');
    if (alerts.data.length > 0) {
      alerts.data.forEach(alert => {
        console.log(`   ⚠️  ${alert.resourceName} - ${alert.message}`);
      });
    } else {
      console.log('   ✅ No active alerts');
    }
  } catch (e) {
    console.log('   Unable to fetch alerts');
  }
  
  console.log('\n✅ Status check complete!');
}

// Run the check
if (require.main === module) {
  checkSystemStatus().catch(console.error);
}

module.exports = { checkSystemStatus };