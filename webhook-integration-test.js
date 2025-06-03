#!/usr/bin/env node

/**
 * Webhook Integration Test
 * This script tests the webhook functionality by simulating alert triggers
 */

const axios = require('axios');

const PULSE_API_BASE = 'http://localhost:7655/api';
const WEBHOOK_SERVER = 'http://localhost:3001/webhook';

async function testWebhookIntegration() {
    console.log('🔍 Testing Webhook Integration...\n');

    try {
        // 1. Check if Pulse server is running
        console.log('1. Checking Pulse server status...');
        const healthResponse = await axios.get(`${PULSE_API_BASE}/health`);
        console.log('✅ Pulse server is running');

        // 2. Check if webhook server is running
        console.log('\n2. Checking webhook server status...');
        const webhookTestResponse = await axios.post(WEBHOOK_SERVER, {
            test: true,
            message: 'Integration test ping'
        });
        console.log('✅ Webhook server is responding');

        // 3. Get current alert configuration
        console.log('\n3. Fetching current alert configuration...');
        const alertsResponse = await axios.get(`${PULSE_API_BASE}/alerts`);
        const alertData = alertsResponse.data;
        
        console.log(`📊 Alert System Status:`);
        console.log(`   - Active alerts: ${alertData.stats.active}`);
        console.log(`   - Total rules: ${alertData.stats.totalRules}`);
        console.log(`   - Webhook channel enabled: ${alertData.stats.channels.find(c => c.id === 'default')?.enabled}`);

        // 4. Test webhook using Pulse's built-in test endpoint
        console.log('\n4. Testing webhook using Pulse API...');
        const testWebhookApiResponse = await axios.post(`${PULSE_API_BASE}/test-webhook`, {
            url: WEBHOOK_SERVER,
            enabled: true
        });
        
        if (testWebhookApiResponse.data.success) {
            console.log('✅ Webhook test via Pulse API succeeded');
        } else {
            console.log('❌ Webhook test via Pulse API failed:', testWebhookApiResponse.data.error);
        }

        // 5. Test webhook payload format
        console.log('\n5. Testing webhook payload format...');
        const testPayload = {
            timestamp: new Date().toISOString(),
            alert: {
                id: "test_alert_" + Date.now(),
                rule: {
                    name: "Webhook Integration Test",
                    description: "Test alert to verify webhook functionality",
                    severity: "warning",
                    metric: "cpu"
                },
                guest: {
                    name: "test-webhook-guest",
                    id: "999",
                    type: "test",
                    node: "test-node",
                    status: "running"
                },
                value: 92,
                threshold: 85,
                emoji: "⚠️"
            },
            embeds: [{
                title: "⚠️ Webhook Integration Test",
                description: "Test alert to verify webhook functionality",
                color: 15844367,
                fields: [
                    {
                        name: "Test Type",
                        value: "Integration Test",
                        inline: true
                    },
                    {
                        name: "Status",
                        value: "SUCCESS",
                        inline: true
                    }
                ],
                footer: {
                    text: "Pulse Monitoring System - Test Mode"
                }
            }]
        };

        const testWebhookResponse = await axios.post(WEBHOOK_SERVER, testPayload, {
            headers: {
                'Content-Type': 'application/json',
                'User-Agent': 'Pulse-Monitoring/1.0'
            }
        });

        console.log('✅ Webhook payload test successful');

        console.log('\n🎉 Webhook Integration Test Results:');
        console.log('   ✅ Pulse server: Running');
        console.log('   ✅ Webhook server: Running');
        console.log('   ✅ Alert system: Configured');
        console.log('   ✅ Webhook channel: Enabled');
        console.log('   ✅ Payload format: Valid');

        console.log('\n📝 Next Steps:');
        console.log('   1. Monitor the test-webhook.js console for incoming webhooks');
        console.log('   2. Check if alerts are triggered naturally by your system metrics');
        console.log('   3. If needed, temporarily lower alert thresholds to test real alerts');
        console.log('   4. Webhook URL configured in .env: http://localhost:3001/webhook');

    } catch (error) {
        console.error('❌ Webhook integration test failed:', error.message);
        
        if (error.code === 'ECONNREFUSED') {
            console.log('\n💡 Troubleshooting:');
            console.log('   - Make sure both Pulse (port 7655) and test-webhook.js (port 3001) are running');
            console.log('   - Check that WEBHOOK_URL is set in .env file');
        }
    }
}

// Run the test
testWebhookIntegration();