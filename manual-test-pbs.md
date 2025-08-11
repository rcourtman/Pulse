# Manual Test Steps for PBS Form Fix

## Test Procedure

1. Open http://192.168.0.123:7655 in browser
2. Navigate to Settings → Nodes tab
3. Click "Add PVE Node"
   - Enter name: test-pve
   - Enter host: https://192.168.1.100:8006
   - Enter token: root@pam!pvetoken
   - Click Cancel (don't save)
4. Click "Add PBS Node" 
   - CHECK: Form should be completely empty (no PVE data)
   - If form has any PVE data = FAIL
5. Add a real PBS node:
   - Name: test-pbs
   - Host: https://192.168.1.200:8007  
   - Token: root@pam!pbstoken
   - Token value: xxxxx
   - Click Add Node
6. Edit the PBS node (click edit icon)
   - CHECK: Form should show PBS data (test-pbs, https://192.168.1.200:8007, etc)
   - If form is empty or shows wrong data = FAIL
7. Cancel and add a PVE node
8. Edit the PVE node
   - CHECK: Form should show PVE data, not PBS data
   - If form shows PBS data = FAIL

## Expected Results
- [ ] PBS form never shows PVE data
- [ ] PVE form never shows PBS data  
- [ ] Editing PBS node shows PBS data
- [ ] Editing PVE node shows PVE data