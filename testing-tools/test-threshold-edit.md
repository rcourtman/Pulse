# Manual Test Plan: Threshold Edit UI Refresh Fix

## Test Objective
Verify that the Save/Cancel buttons remain visible during threshold editing when the UI refreshes every 5 seconds.

## Prerequisites
1. Pulse v4.2.0+ with the fix applied
2. At least one node configured
3. Browser with developer tools

## Test Steps

### Setup
1. Open Pulse in browser (http://localhost:7655)
2. Navigate to Alerts page
3. Click on "Thresholds" tab

### Test Case 1: Create Override and Test Edit Persistence
1. Add a custom threshold override:
   - Click "Add Override" 
   - Select a node or VM
   - Set custom thresholds
   - Save

2. Start editing the override:
   - Click "Edit" button next to the override
   - **Expected**: Save and Cancel buttons appear
   - **Expected**: Threshold sliders become editable

3. Wait for UI refresh (15 seconds total - 3 refresh cycles):
   - Watch the UI (you may see slight data updates)
   - **Expected**: Save and Cancel buttons REMAIN VISIBLE
   - **Expected**: Edit mode is maintained
   - **Expected**: Any changes to sliders are preserved

4. Make a change and save:
   - Adjust one of the threshold sliders
   - Click "Save"
   - **Expected**: Changes are saved
   - **Expected**: Returns to view mode with Edit button

### Test Case 2: Cancel During Refresh
1. Click "Edit" on an override
2. Wait 7-8 seconds (through at least one refresh)
3. Click "Cancel"
   - **Expected**: Returns to view mode
   - **Expected**: No changes are saved

### Test Case 3: Multiple Overrides
1. Create 2-3 overrides
2. Edit one override
3. Wait for refresh
   - **Expected**: Only the one being edited shows Save/Cancel
   - **Expected**: Other overrides still show Edit button

## Verification in Browser Console
Open browser dev tools and run:
```javascript
// Check if edit state is preserved
setInterval(() => {
  const saveBtn = document.querySelector('button:has-text("Save")');
  const editBtn = document.querySelector('button:has-text("Edit")');
  console.log('Save visible:', !!saveBtn, 'Edit visible:', !!editBtn);
}, 1000);
```

## Expected Results
- ✅ Edit state persists across all UI refresh cycles
- ✅ Save/Cancel buttons remain visible during entire edit session
- ✅ Threshold values being edited are not reset during refresh
- ✅ Only the override being edited maintains edit state

## Known Issues (Before Fix)
- ❌ Save/Cancel buttons disappeared after 5-second refresh
- ❌ Users lost ability to save changes
- ❌ Had to be very quick to save before refresh

## Fix Implementation
The fix tracks editing state at the parent component level (ThresholdsTab) instead of locally in each OverrideItem, preventing state loss during re-renders.