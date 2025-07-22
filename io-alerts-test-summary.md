# I/O Alerts Feature Summary

## Implemented Features

### 1. 30-second Sustained Period for I/O Metrics
- I/O metrics (disk read/write, network in/out) must exceed threshold for 30 continuous seconds before triggering
- Visual progress bar shows countdown during pending state
- Different from CPU/memory/disk which can trigger instantly

### 2. Configurable Sustained Period
- Added `ioSustainedPeriod` field to alert rules JSON (default: 30000ms)
- UI dropdown in alert settings to choose: 10s, 30s, 1m, 2m, 5m
- Allows users to adjust sensitivity based on their needs

### 3. Visual Indicators for Pending I/O Alerts
- Yellow progress bar shows time remaining until alert triggers
- Updates in real-time as metrics are received
- Clear visual feedback during the sustained period

### 4. Hysteresis (20% Buffer)
- Prevents alert flapping when values hover around threshold
- Alert triggers at 100% of threshold
- Alert only resolves when value drops below 80% of threshold
- Example: 10 MB/s threshold - triggers at 10 MB/s, resolves at 8 MB/s

### 5. 5-Minute Rolling Average
- I/O rates calculated using 5-minute rolling average
- Smooths out brief spikes for more accurate readings
- Stores up to 30 samples (5 minutes at 10-second intervals)
- Debug logging shows both instant and average rates

### 6. 2-Minute Cooldown After Resolution
- Prevents immediate re-triggering after an alert resolves
- Gives system time to stabilize after high I/O activity
- Cooldown only applies to resolved alerts, not new metrics

## Testing the Features

### Test 1: Sustained Period
1. Set a low threshold (e.g., 1 MB/s for network out)
2. Generate traffic that exceeds threshold
3. Observe the yellow progress bar counting down from 30s
4. Alert triggers only after full 30s has elapsed

### Test 2: Hysteresis
1. Create an active I/O alert
2. Reduce traffic to between 80-100% of threshold
3. Alert should remain active (within hysteresis band)
4. Reduce traffic below 80% of threshold
5. Alert should resolve

### Test 3: Rolling Average
1. Generate brief spikes of I/O activity
2. Monitor logs for instant vs 5-min average rates
3. Alert decisions based on average, not spikes

### Test 4: Configurable Period
1. Change I/O sustained period in UI settings
2. Save configuration
3. New alerts will use the updated period

## Configuration Example

```json
{
  "per-guest-alerts": {
    "condition": "greater_than",
    "duration": 0,
    "ioSustainedPeriod": 30000,
    "globalThresholds": {
      "cpu": 80,
      "memory": 85,
      "disk": 90,
      "diskread": "",
      "diskwrite": "",
      "netin": "",
      "netout": ""
    },
    "guestThresholds": {
      "primary-node-vmid": {
        "netout": "10485760"  // 10 MB/s
      }
    }
  }
}
```

## Benefits

1. **Reduced False Positives**: Brief I/O spikes don't trigger alerts
2. **Stable Alerts**: Hysteresis prevents flapping
3. **Accurate Measurements**: Rolling average provides true I/O patterns
4. **User Control**: Configurable periods for different environments
5. **Clear Feedback**: Visual indicators show alert state