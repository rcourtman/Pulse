const axios = require('axios');
const { spawn } = require('child_process');

async function checkUIRequest() {
  console.log('=== CHECKING UI REQUEST ===\n');
  
  console.log('Starting log monitor...');
  const logProcess = spawn('sudo', ['tail', '-f', '/opt/pulse/pulse.log']);
  
  let capturedLogs = '';
  
  logProcess.stdout.on('data', (data) => {
    const output = data.toString();
    capturedLogs += output;
    if (output.includes('Test notification request') || output.includes('Testing email with provided config')) {
      process.stdout.write(output);
    }
  });
  
  // Wait for log monitor to start
  await new Promise(resolve => setTimeout(resolve, 1000));
  
  console.log('\nPlease click "Send Test Email" in the UI now...\n');
  console.log('Waiting for request (30 seconds)...\n');
  
  // Wait for user to trigger the test
  await new Promise(resolve => setTimeout(resolve, 30000));
  
  logProcess.kill();
  
  // Check if we captured the request
  if (capturedLogs.includes('Test notification request')) {
    console.log('\n✅ Found test request in logs');
    
    // Extract the request body
    const bodyMatch = capturedLogs.match(/body=({.*?}) msg=/);
    if (bodyMatch) {
      try {
        const body = JSON.parse(bodyMatch[1]);
        console.log('\nRequest body:', JSON.stringify(body, null, 2));
      } catch (e) {
        console.log('Could not parse body');
      }
    }
  } else {
    console.log('\n❌ No test request found in logs');
    console.log('Make sure you clicked "Send Test Email" in the UI');
  }
}

checkUIRequest().catch(console.error);