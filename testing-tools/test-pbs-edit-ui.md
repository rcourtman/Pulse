# PBS Edit Form Test Instructions

## Test Date: 2025-08-12

## Test Objective
Verify that PBS node edit forms correctly load and save configuration data, especially token authentication details.

## Current PBS Node Data
Based on API response, we have a PBS node with:
- **ID**: pbs-0
- **Name**: pbs-docker
- **Host**: https://192.168.0.8:8007
- **Auth Type**: Token (hasToken: true, hasPassword: false)
- **Token Name**: pulse-monitor@pbs!pulse-192-168-0-123-1754983958
- **Monitoring Settings**: All enabled except monitorGarbageJobs

## Test Steps

### 1. Open Edit Modal
1. Navigate to http://localhost:7655
2. Go to Settings → Nodes tab
3. Find the PBS node "pbs-docker"
4. Click the Edit button (pencil icon)

### 2. Verify Form Population
Check that the following fields are correctly populated:

#### Basic Fields
- [ ] **Name**: Should show "pbs-docker"
- [ ] **Host URL**: Should show "https://192.168.0.8:8007"
- [ ] **Verify SSL**: Should be unchecked (based on verifySSL: false)

#### Authentication Fields
- [ ] **Auth Type**: Token option should be selected
- [ ] **Token ID field**: Should show FULL token format "pulse-monitor@pbs!pulse-192-168-0-123-1754983958"
- [ ] **Token Value field**: Should be empty (password fields never show existing values)

#### Monitoring Options
- [ ] **Monitor Datastores**: Should be checked ✓
- [ ] **Monitor Sync Jobs**: Should be checked ✓
- [ ] **Monitor Verify Jobs**: Should be checked ✓
- [ ] **Monitor Prune Jobs**: Should be checked ✓
- [ ] **Monitor Garbage Collection Jobs**: Should be unchecked ✗

### 3. Test Saving Without Changes
1. Click "Save" without making any changes
2. Verify the node continues to work
3. Check that monitoring continues normally

### 4. Test Minor Edit
1. Open edit modal again
2. Change the name to "PBS Docker Updated"
3. Click Save
4. Verify the name updates in the list
5. Verify monitoring continues working

### 5. Test Token Auth Preservation
1. Open edit modal again
2. Verify Token ID still shows full format
3. Add a space at the end of Token ID, then remove it
4. Click Save
5. Verify node still connects properly

## Expected Behavior

### Correct Token Handling
- When editing a PBS node with token auth, the Token ID field should display the FULL token format including username
- Format: `username@realm!token-name`
- Example: `pulse-monitor@pbs!pulse-192-168-0-123-1754983958`

### What NOT to expect
- Token Value will never be shown (security feature)
- Password fields are always empty when editing

## Known Issues Fixed
- PBS edit form now correctly loads the full token ID
- Token authentication type is properly detected
- Monitoring settings are correctly populated

## API Verification
Run this command to verify the node data:
```bash
curl -s "http://localhost:7655/api/config/nodes" | jq '.[] | select(.type == "pbs")'
```

Expected fields:
- `tokenName`: Full format with username
- `hasToken`: true
- `hasPassword`: false

## Success Criteria
- [ ] All form fields populate correctly when editing
- [ ] Saving without changes doesn't break the node
- [ ] Token ID shows full format including username
- [ ] Monitoring settings are preserved correctly
- [ ] Node continues to function after editing