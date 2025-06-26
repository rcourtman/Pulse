const fs = require('fs');
const path = require('path');

// Read state directly from file
const statePath = path.join(__dirname, 'state.json');
if (!fs.existsSync(statePath)) {
    console.log('No state.json file found');
    process.exit(1);
}

const state = JSON.parse(fs.readFileSync(statePath, 'utf8'));

console.log('VMs by node:');
const nodeMap = {};
(state.vms || []).forEach(vm => {
    if (!nodeMap[vm.node]) nodeMap[vm.node] = [];
    nodeMap[vm.node].push({
        vmid: vm.vmid,
        name: vm.name,
        endpointId: vm.endpointId || 'primary'
    });
});

Object.entries(nodeMap).forEach(([node, vms]) => {
    console.log(`  Node '${node}': ${vms.length} VMs`);
    vms.slice(0, 3).forEach(vm => {
        console.log(`    - VM ${vm.vmid}: ${vm.name} (endpoint: ${vm.endpointId})`);
    });
});

console.log('\nPBS owner tokens:');
const ownerTokens = new Set();
(state.pbs || []).forEach(pbs => {
    (pbs.datastores || []).forEach(ds => {
        (ds.snapshots || []).forEach(snap => {
            if (snap.owner) ownerTokens.add(snap.owner);
        });
    });
});

[...ownerTokens].slice(0, 10).forEach(owner => {
    console.log(`  - ${owner}`);
});

console.log('\nPBS backup to node mapping:');
const backupNodeMap = {};
(state.pbs || []).forEach(pbs => {
    (pbs.datastores || []).forEach(ds => {
        (ds.snapshots || []).forEach(snap => {
            const token = snap.owner ? snap.owner.split('!')[1] : 'unknown';
            const key = `${snap['backup-type']}/${snap['backup-id']}`;
            if (!backupNodeMap[key]) backupNodeMap[key] = [];
            backupNodeMap[key].push({
                owner: snap.owner,
                token: token,
                namespace: snap.namespace || 'root'
            });
        });
    });
});

// Show first few backups and what nodes they might map to
Object.entries(backupNodeMap).slice(0, 5).forEach(([key, backups]) => {
    console.log(`  Backup ${key}:`);
    backups.forEach(b => {
        console.log(`    - Owner: ${b.owner} (token: ${b.token})`);
    });
    
    // Find matching VMs
    const [type, vmid] = key.split('/');
    const matchingVMs = (state.vms || []).filter(vm => String(vm.vmid) === vmid);
    if (matchingVMs.length > 0) {
        console.log(`    - Matching VMs:`);
        matchingVMs.forEach(vm => {
            console.log(`      -> Node: ${vm.node}, Name: ${vm.name}`);
        });
    }
});