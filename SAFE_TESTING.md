# Safe Testing Guide for Pulse

## The Problem (SOLVED)
Tests were deleting production nodes when cleaning up test data. This has been fixed!

## The Solution
We've implemented multiple layers of protection:

### 1. Mock Mode Testing (`run-tests-mock.sh`)
**RECOMMENDED** - Use this for all testing:
```bash
./scripts/run-tests-mock.sh
```

This script:
- Automatically enables mock mode before tests
- Runs all tests against fake nodes (pve1-pve7)
- Restores your original mode when done
- **Your production nodes are NEVER touched**

### 2. Safe Test Helpers
All test scripts now use `test-helpers.sh` which:
- **NEVER deletes nodes matching**: pve*, mock-*, delly, minipc, pimox, 192.168.0.*
- Only deletes nodes with "test" in the name
- Double-checks before any deletion

### 3. Safety Prompt in Main Test
If you run `./scripts/run-tests.sh` in real mode:
- Shows a BIG WARNING
- Asks for confirmation
- Recommends using mock mode instead

## Quick Commands

### Safe Testing (Recommended)
```bash
# Run tests safely with mock data
./scripts/run-tests-mock.sh

# Check what mode you're in
/opt/pulse/scripts/toggle-mock.sh status

# Switch to mock mode manually
/opt/pulse/scripts/toggle-mock.sh on

# Switch back to real nodes
/opt/pulse/scripts/toggle-mock.sh off
```

### Configure Mock Data
```bash
# Edit mock settings (node count, VMs, etc)
/opt/pulse/scripts/toggle-mock.sh edit

# Then restart to apply
sudo systemctl restart pulse-dev
```

## Protected Nodes
These patterns are ALWAYS protected from deletion:
- `pve[0-9]` - Mock nodes
- `mock-*` - Any mock-prefixed nodes
- `delly` - Production node
- `minipc` - Production node  
- `pimox` - Production node
- `192.168.0.*` - Production IP range

## Test Node Naming
Test scripts now create nodes with unique names:
- `test-val-[timestamp]`
- `persist-test-[timestamp]-[random]`
- `load-test-[timestamp]`
- `concurrent-[number]`

This prevents any collision with real node names.

## Files Modified
- `/opt/pulse/scripts/run-tests.sh` - Added safety prompt
- `/opt/pulse/scripts/run-tests-mock.sh` - New safe test runner
- `/opt/pulse/scripts/test-helpers.sh` - Safe deletion functions
- `/opt/pulse/scripts/test-persistence.sh` - Uses safe helpers
- `/opt/pulse/scripts/test-recovery.sh` - Uses safe helpers
- `/opt/pulse/scripts/test-backup.sh` - Uses safe helpers
- `/opt/pulse/scripts/test-load.sh` - Uses safe helpers
- `/opt/pulse/scripts/test-config-validation.sh` - Uses safe helpers

## Your Production Nodes Are Safe!
The test suite will never again delete your production nodes. Tests now:
1. Use mock data by default (recommended)
2. Only delete nodes explicitly created for testing
3. Protect all known production node patterns
4. Ask for confirmation before running in real mode