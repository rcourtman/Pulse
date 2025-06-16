#!/usr/bin/env node

/**
 * PBS Namespace Diagnostic Tool
 * 
 * This script probes your PBS setup to understand:
 * 1. What namespaces exist
 * 2. What snapshots are in each namespace  
 * 3. What backup tasks exist and their namespace assignment
 * 
 * Usage: node debug-pbs-namespaces.js
 */

const https = require('https');
const axios = require('axios');
require('dotenv').config();

// Configure axios to ignore SSL certificates (like Pulse does)
const httpsAgent = new https.Agent({
    rejectUnauthorized: false
});

async function main() {
    console.log('🔍 PBS Namespace Diagnostic Tool\n');
    
    // Get PBS config from environment
    const pbsHost = process.env.PBS_HOST;
    const pbsPort = process.env.PBS_PORT || 8007;
    const pbsTokenId = process.env.PBS_TOKEN_ID;
    const pbsTokenSecret = process.env.PBS_TOKEN_SECRET;
    
    if (!pbsHost || !pbsTokenId || !pbsTokenSecret) {
        console.error('❌ Missing PBS configuration in .env file:');
        console.error('   PBS_HOST, PBS_TOKEN_ID, PBS_TOKEN_SECRET are required');
        console.error('   PBS_PORT is optional (defaults to 8007)');
        process.exit(1);
    }
    
    console.log(`📡 Connecting to PBS: ${pbsHost}:${pbsPort}`);
    console.log(`👤 Token: ${pbsTokenId}`);
    console.log('');
    
    const baseURL = `https://${pbsHost}:${pbsPort}/api2/json`;
    const client = axios.create({
        baseURL,
        httpsAgent,
        headers: {
            'Authorization': `PBSAPIToken=${pbsTokenId}:${pbsTokenSecret}`
        }
    });
    
    try {
        // 1. Get datastores
        console.log('📊 Fetching datastores...');
        const datastoresResponse = await client.get('/admin/datastore');
        const datastores = datastoresResponse.data?.data || [];
        console.log(`Found ${datastores.length} datastores: ${datastores.map(ds => ds.store).join(', ')}\n`);
        
        // 2. For each datastore, get namespaces
        for (const datastore of datastores) {
            console.log(`🗂️  Analyzing datastore: ${datastore.store}`);
            
            try {
                // Get namespaces for this datastore
                const namespacesResponse = await client.get(`/admin/datastore/${datastore.store}/namespace`);
                const namespaces = namespacesResponse.data?.data || [];
                
                if (namespaces.length === 0) {
                    console.log('   📁 Namespaces: root only');
                    await analyzeNamespace(client, datastore.store, null);
                } else {
                    console.log(`   📁 Namespaces: ${namespaces.map(ns => ns.ns).join(', ')}`);
                    
                    // Analyze root namespace (empty string)
                    await analyzeNamespace(client, datastore.store, '');
                    
                    // Analyze each named namespace
                    for (const namespace of namespaces) {
                        await analyzeNamespace(client, datastore.store, namespace.ns);
                    }
                }
            } catch (nsError) {
                console.log(`   ⚠️  Could not fetch namespaces: ${nsError.message}`);
                // Try root namespace anyway
                await analyzeNamespace(client, datastore.store, null);
            }
            console.log('');
        }
        
        // 3. Get recent backup tasks from node
        console.log('📋 Fetching recent backup tasks from node...');
        try {
            const nodesResponse = await client.get('/nodes');
            const nodes = nodesResponse.data?.data || [];
            
            if (nodes.length > 0) {
                const node = nodes[0].node;
                console.log(`   Using node: ${node}`);
                
                const tasksResponse = await client.get(`/nodes/${node}/tasks`, {
                    params: { 
                        since: Math.floor(Date.now() / 1000) - (7 * 24 * 60 * 60), // Last 7 days
                        limit: 100
                    }
                });
                const tasks = tasksResponse.data?.data || [];
                const backupTasks = tasks.filter(task => 
                    (task.worker_type === 'backup' || task.type === 'backup') && task.worker_id
                );
                
                console.log(`   Found ${backupTasks.length} backup tasks in last 7 days:`);
                backupTasks.slice(0, 10).forEach(task => {
                    console.log(`     • ${task.worker_id} - ${task.status} (${new Date(task.starttime * 1000).toLocaleString()})`);
                });
                if (backupTasks.length > 10) {
                    console.log(`     ... and ${backupTasks.length - 10} more`);
                }
            }
        } catch (taskError) {
            console.log(`   ⚠️  Could not fetch backup tasks: ${taskError.message}`);
        }
        
    } catch (error) {
        console.error(`❌ Connection failed: ${error.message}`);
        if (error.response?.status === 401) {
            console.error('   Check your PBS_USER and PBS_TOKEN credentials');
        }
        process.exit(1);
    }
}

async function analyzeNamespace(client, datastoreName, namespace) {
    const nsDisplay = namespace || 'root';
    const nsParam = namespace || '';
    
    try {
        // Get groups in this namespace
        const groupsResponse = await client.get(`/admin/datastore/${datastoreName}/groups`, {
            params: { ns: nsParam }
        });
        const groups = groupsResponse.data?.data || [];
        
        if (groups.length === 0) {
            console.log(`     📂 ${nsDisplay}: no backup groups`);
            return;
        }
        
        console.log(`     📂 ${nsDisplay}: ${groups.length} backup groups`);
        
        // Get snapshots for each group to see recent activity
        let totalSnapshots = 0;
        let recentSnapshots = 0;
        const cutoff = Math.floor(Date.now() / 1000) - (7 * 24 * 60 * 60); // 7 days ago
        
        for (const group of groups.slice(0, 5)) { // Limit to first 5 groups for brevity
            try {
                const snapshotsResponse = await client.get(`/admin/datastore/${datastoreName}/snapshots`, {
                    params: {
                        'backup-type': group['backup-type'],
                        'backup-id': group['backup-id'],
                        ns: nsParam
                    }
                });
                const snapshots = snapshotsResponse.data?.data || [];
                totalSnapshots += snapshots.length;
                
                const recent = snapshots.filter(snap => snap['backup-time'] >= cutoff).length;
                recentSnapshots += recent;
                
                if (recent > 0) {
                    console.log(`       • ${group['backup-type']}/${group['backup-id']}: ${recent} snapshots in last 7 days`);
                }
            } catch (snapError) {
                // Ignore snapshot errors for individual groups
            }
        }
        
        if (groups.length > 5) {
            console.log(`       ... and ${groups.length - 5} more groups`);
        }
        
        console.log(`       📈 Total: ${recentSnapshots} recent snapshots (${totalSnapshots} total)`);
        
    } catch (error) {
        console.log(`     📂 ${nsDisplay}: error - ${error.message}`);
    }
}

if (require.main === module) {
    main().catch(console.error);
}