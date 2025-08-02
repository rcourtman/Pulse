const axios = require('axios');

const API_BASE = 'http://localhost:3000/api';

async function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function testAlertsAPI() {
  console.log('=== TESTING ALERTS API DIRECTLY ===\n');
  
  try {
    // 1. Get current configuration
    console.log('1. Current Alert Configuration:');
    const config = await axios.get(`${API_BASE}/alerts/config`);
    const thresholds = config.data.guestDefaults;
    
    console.log('   CPU Threshold:', thresholds.cpu.trigger + '%');
    console.log('   Memory Threshold:', thresholds.memory.trigger + '%');
    console.log('   Disk Threshold:', thresholds.disk.trigger + '%');
    
    // 2. Get current alerts
    console.log('\n2. Current Active Alerts:');
    const alerts = await axios.get(`${API_BASE}/alerts/active`);
    console.log(`   Total alerts: ${alerts.data.length}`);
    
    // Group alerts by type
    const alertsByType = {};
    alerts.data.forEach(alert => {
      const type = alert.metric || alert.type || 'unknown';
      if (!alertsByType[type]) alertsByType[type] = [];
      alertsByType[type].push(alert);
    });
    
    Object.entries(alertsByType).forEach(([type, typeAlerts]) => {
      console.log(`\n   ${type} alerts (${typeAlerts.length}):`);
      typeAlerts.slice(0, 3).forEach(alert => {
        console.log(`     - ${alert.resourceName}: ${alert.message}`);
        console.log(`       Value: ${alert.value?.toFixed(1)}%, ID: ${alert.id}`);
      });
      if (typeAlerts.length > 3) {
        console.log(`     ... and ${typeAlerts.length - 3} more`);
      }
    });
    
    // 3. Test changing thresholds
    console.log('\n3. Testing Threshold Changes:');
    
    // Save original config
    const originalConfig = JSON.parse(JSON.stringify(config.data));
    
    // Lower CPU threshold to trigger alerts
    console.log('\n   Lowering CPU threshold to 5%...');
    config.data.guestDefaults.cpu.trigger = 5;
    config.data.guestDefaults.cpu.clear = 3;
    
    await axios.put(`${API_BASE}/alerts/config`, config.data);
    console.log('   ✅ Configuration updated');
    
    // Wait for alert system to react
    console.log('   Waiting 10 seconds for alerts to generate...');
    await sleep(10000);
    
    // Check new alerts
    const newAlerts = await axios.get(`${API_BASE}/alerts/active`);
    const cpuAlerts = newAlerts.data.filter(a => 
      (a.metric && a.metric.toLowerCase() === 'cpu') || 
      (a.message && a.message.toLowerCase().includes('cpu'))
    );
    
    console.log(`\n   CPU alerts found: ${cpuAlerts.length}`);
    if (cpuAlerts.length > 0) {
      console.log('   Sample CPU alerts:');
      cpuAlerts.slice(0, 5).forEach(alert => {
        console.log(`     - ${alert.resourceName}: CPU at ${alert.value?.toFixed(1)}%`);
      });
    }
    
    // 4. Test memory threshold
    console.log('\n   Lowering Memory threshold to 10%...');
    config.data.guestDefaults.memory.trigger = 10;
    config.data.guestDefaults.memory.clear = 8;
    
    await axios.put(`${API_BASE}/alerts/config`, config.data);
    await sleep(10000);
    
    const memAlerts = await axios.get(`${API_BASE}/alerts/active`);
    const memoryAlerts = memAlerts.data.filter(a => 
      (a.metric && a.metric.toLowerCase() === 'memory') || 
      (a.message && a.message.toLowerCase().includes('memory'))
    );
    
    console.log(`\n   Memory alerts found: ${memoryAlerts.length}`);
    if (memoryAlerts.length > 0) {
      console.log('   Sample Memory alerts:');
      memoryAlerts.slice(0, 5).forEach(alert => {
        console.log(`     - ${alert.resourceName}: Memory at ${alert.value?.toFixed(1)}%`);
      });
    }
    
    // 5. Test acknowledging alerts
    console.log('\n4. Testing Alert Acknowledgement:');
    if (cpuAlerts.length > 0) {
      const testAlert = cpuAlerts[0];
      console.log(`   Acknowledging alert: ${testAlert.id}`);
      
      try {
        await axios.post(`${API_BASE}/alerts/${testAlert.id}/acknowledge`);
        console.log('   ✅ Alert acknowledged successfully');
        
        // Verify acknowledgement
        const checkAlerts = await axios.get(`${API_BASE}/alerts/active`);
        const ackAlert = checkAlerts.data.find(a => a.id === testAlert.id);
        if (ackAlert && ackAlert.acknowledged) {
          console.log('   ✅ Acknowledgement verified');
        }
      } catch (e) {
        console.log('   ❌ Failed to acknowledge:', e.response?.data || e.message);
      }
    }
    
    // 6. Restore original configuration
    console.log('\n5. Restoring Original Configuration:');
    await axios.put(`${API_BASE}/alerts/config`, originalConfig);
    console.log('   ✅ Configuration restored');
    
    // Final check
    console.log('\n   Waiting 10 seconds for alerts to clear...');
    await sleep(10000);
    
    const finalAlerts = await axios.get(`${API_BASE}/alerts/active`);
    const activeCpuAlerts = finalAlerts.data.filter(a => 
      !a.acknowledged && 
      ((a.metric && a.metric.toLowerCase() === 'cpu') || 
       (a.message && a.message.toLowerCase().includes('cpu')))
    );
    
    console.log(`\n   Final active CPU alerts: ${activeCpuAlerts.length}`);
    console.log('   Total active alerts: ' + finalAlerts.data.filter(a => !a.acknowledged).length);
    
  } catch (error) {
    console.error('\n❌ Test error:', error.response?.data || error.message);
  }
}

// Run the test
if (require.main === module) {
  testAlertsAPI().catch(console.error);
}

module.exports = { testAlertsAPI };