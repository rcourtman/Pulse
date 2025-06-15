#!/usr/bin/env node

const axios = require('axios');
const https = require('https');
require('dotenv').config({ path: '/opt/pulse/.env' });

async function testPbsApi() {
    // Get PBS config from environment
    const pbsHost = process.env.PBS_HOST;
    const pbsPort = process.env.PBS_PORT || 8007;
    const tokenId = process.env.PBS_TOKEN_ID;
    const tokenSecret = process.env.PBS_TOKEN_SECRET;
    
    if (!pbsHost || !tokenId || !tokenSecret) {
        console.error('PBS configuration not found in environment');
        return;
    }
    
    // Create axios client
    const client = axios.create({
        baseURL: `https://${pbsHost}:${pbsPort}/api2/json`,
        headers: {
            'Authorization': `PBSAPIToken=${tokenId}:${tokenSecret}`
        },
        httpsAgent: new https.Agent({ rejectUnauthorized: false })
    });
    
    try {
        console.log('=== Testing PBS API Endpoints ===\n');
        
        // 1. Get datastores
        console.log('1. Getting datastores...');
        const datastoresResp = await client.get('/admin/datastore');
        const datastores = datastoresResp.data?.data || [];
        console.log(`Found ${datastores.length} datastores:`, datastores.map(d => d.store));
        
        if (datastores.length === 0) {
            console.log('No datastores found!');
            return;
        }
        
        const datastoreName = datastores[0].store;
        console.log(`\nUsing datastore: ${datastoreName}\n`);
        
        // 2. Try to list namespaces
        console.log('2. Trying to list namespaces...');
        try {
            const nsResp = await client.get(`/admin/datastore/${datastoreName}/namespace`);
            const namespaces = nsResp.data?.data || [];
            console.log(`Found ${namespaces.length} namespaces:`, namespaces);
        } catch (error) {
            console.log('Namespace endpoint not available:', error.response?.status || error.message);
        }
        
        // 3. Get groups without namespace
        console.log('\n3. Getting groups (no namespace param)...');
        const groupsResp = await client.get(`/admin/datastore/${datastoreName}/groups`);
        const groups = groupsResp.data?.data || [];
        console.log(`Found ${groups.length} groups`);
        if (groups.length > 0) {
            console.log('First group:', JSON.stringify(groups[0], null, 2));
            console.log('Group fields:', Object.keys(groups[0]));
        }
        
        // 4. Get groups with root namespace
        console.log('\n4. Getting groups with ns="" (root)...');
        const rootGroupsResp = await client.get(`/admin/datastore/${datastoreName}/groups`, {
            params: { ns: '' }
        });
        const rootGroups = rootGroupsResp.data?.data || [];
        console.log(`Found ${rootGroups.length} groups in root namespace`);
        
        // 5. Get groups with pimox namespace
        console.log('\n5. Getting groups with ns="pimox"...');
        try {
            const pimoxGroupsResp = await client.get(`/admin/datastore/${datastoreName}/groups`, {
                params: { ns: 'pimox' }
            });
            const pimoxGroups = pimoxGroupsResp.data?.data || [];
            console.log(`Found ${pimoxGroups.length} groups in pimox namespace`);
            console.log('Pimox namespace groups:', pimoxGroups.map(g => `${g['backup-type']}/${g['backup-id']}`));
        } catch (error) {
            console.log('Error getting pimox groups:', error.response?.status || error.message);
        }
        
        // 6. Get snapshots without namespace
        console.log('\n6. Getting all snapshots (no namespace param)...');
        const allSnapsResp = await client.get(`/admin/datastore/${datastoreName}/snapshots`);
        const allSnaps = allSnapsResp.data?.data || [];
        console.log(`Found ${allSnaps.length} total snapshots`);
        if (allSnaps.length > 0) {
            console.log('First snapshot:', JSON.stringify(allSnaps[0], null, 2));
            console.log('Snapshot fields:', Object.keys(allSnaps[0]));
        }
        
        // 7. Get snapshots with namespace
        console.log('\n7. Getting snapshots with ns="pimox"...');
        const pimoxSnapsResp = await client.get(`/admin/datastore/${datastoreName}/snapshots`, {
            params: { ns: 'pimox' }
        });
        const pimoxSnaps = pimoxSnapsResp.data?.data || [];
        console.log(`Found ${pimoxSnaps.length} snapshots in pimox namespace`);
        
        // 8. Try content endpoint which might show namespace structure
        console.log('\n8. Trying content endpoint...');
        try {
            const contentResp = await client.get(`/admin/datastore/${datastoreName}/content`, {
                params: { ns: '' }
            });
            const content = contentResp.data?.data || [];
            console.log(`Found ${content.length} items in content`);
            if (content.length > 0) {
                console.log('First content item:', JSON.stringify(content[0], null, 2));
            }
        } catch (error) {
            console.log('Content endpoint error:', error.response?.status || error.message);
        }
        
        // 9. Try to get a specific group's snapshots in different namespaces
        if (groups.length > 0) {
            const testGroup = groups[0];
            console.log(`\n9. Testing specific group ${testGroup['backup-type']}/${testGroup['backup-id']}...`);
            
            // Root namespace
            const rootSnapResp = await client.get(`/admin/datastore/${datastoreName}/snapshots`, {
                params: {
                    'backup-type': testGroup['backup-type'],
                    'backup-id': testGroup['backup-id'],
                    ns: ''
                }
            });
            console.log(`  Root namespace: ${rootSnapResp.data?.data?.length || 0} snapshots`);
            
            // Pimox namespace
            try {
                const pimoxSnapResp = await client.get(`/admin/datastore/${datastoreName}/snapshots`, {
                    params: {
                        'backup-type': testGroup['backup-type'],
                        'backup-id': testGroup['backup-id'],
                        ns: 'pimox'
                    }
                });
                console.log(`  Pimox namespace: ${pimoxSnapResp.data?.data?.length || 0} snapshots`);
            } catch (error) {
                console.log(`  Pimox namespace error: ${error.response?.status || error.message}`);
            }
        }
        
        // 10. Try to find namespace info in different ways
        console.log('\n10. Looking for namespace indicators...');
        
        // Check if any groups have namespace in owner field
        const ownersWithSlash = groups.filter(g => g.owner && g.owner.includes('!'));
        if (ownersWithSlash.length > 0) {
            console.log(`Found ${ownersWithSlash.length} groups with '!' in owner field:`);
            ownersWithSlash.slice(0, 5).forEach(g => {
                console.log(`  ${g['backup-type']}/${g['backup-id']}: owner = ${g.owner}`);
            });
        }
        
    } catch (error) {
        console.error('Error testing PBS API:', error.message);
        if (error.response) {
            console.error('Response status:', error.response.status);
            console.error('Response data:', error.response.data);
        }
    }
}

testPbsApi().catch(console.error);