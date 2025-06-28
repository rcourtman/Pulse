# PBS Backups API Documentation

## Overview
A dedicated REST API for querying and analyzing PBS backup data, including namespace grouping, collision detection, and guest-specific queries.

## Endpoints

### 1. GET `/api/pbs/backups`
Get all backup data across all PBS instances with namespace grouping and collision analysis.

**Response:**
```json
{
  "success": true,
  "data": {
    "instances": [
      {
        "id": "pbs_primary_token",
        "name": "192.168.0.16",
        "status": "ok",
        "namespaces": {
          "root": {
            "vms": {
              "100": {
                "id": "100",
                "backups": [...],
                "totalSize": 123456789,
                "latestBackup": 1751079650
              }
            },
            "cts": {...},
            "totalBackups": 187,
            "totalSize": 1234567890,
            "oldestBackup": 1740000000,
            "newestBackup": 1751079650
          },
          "pimox": {...}
        },
        "datastores": [...],
        "collisions": [...],
        "stats": {
          "totalBackups": 231,
          "totalSize": 1363132347876,
          "totalGuests": 21
        }
      }
    ],
    "globalStats": {
      "totalBackups": 231,
      "totalSize": 1363132347876,
      "totalGuests": 21,
      "namespaces": ["root", "pimox"],
      "collisions": []
    }
  }
}
```

### 2. GET `/api/pbs/backups/:instanceId`
Get backup data for a specific PBS instance.

**Parameters:**
- `instanceId`: PBS instance ID (e.g., "pbs_primary_token")

**Response:** Same structure as above but for single instance

### 3. GET `/api/pbs/backups/guest/:type/:id`
Get all backups for a specific guest across all PBS instances and namespaces.

**Parameters:**
- `type`: Guest type - "vm" or "ct"
- `id`: Guest ID (e.g., "100")

**Response:**
```json
{
  "success": true,
  "data": {
    "guestType": "ct",
    "guestId": "100",
    "instances": [
      {
        "instanceId": "pbs_primary_token",
        "instanceName": "192.168.0.16",
        "namespaces": {
          "root": {
            "datastore": "main",
            "backups": [
              {
                "time": 1751079650,
                "size": 123456789,
                "verified": true,
                "comment": "pihole, pi, 100"
              }
            ]
          }
        }
      }
    ],
    "totalBackups": 22,
    "totalSize": 2468013578,
    "oldestBackup": 1740000000,
    "newestBackup": 1751079650
  }
}
```

### 4. GET `/api/pbs/collisions`
Get VMID collision analysis across all PBS instances.

**Response:**
```json
{
  "success": true,
  "data": {
    "hasCollisions": true,
    "totalCollisions": 16,
    "criticalCollisions": 0,
    "warningCollisions": 16,
    "byInstance": [
      {
        "instanceId": "pbs_primary_token",
        "instanceName": "192.168.0.16",
        "collisions": {
          "detected": true,
          "totalCollisions": 16,
          "byDatastore": {
            "main": {
              "hasCollisions": true,
              "collisions": [
                {
                  "vmid": "ct/100",
                  "namespace": "root",
                  "totalSnapshots": 22,
                  "sources": [
                    {
                      "comment": "pihole, pi, 100",
                      "count": 11,
                      "lastBackup": "2025-06-28T12:00:17.000Z"
                    }
                  ],
                  "latestOwner": "pihole, pi, 100",
                  "oldestSnapshot": "2025-05-15T01:00:01.000Z",
                  "newestSnapshot": "2025-06-28T12:00:17.000Z"
                }
              ],
              "warnings": [...]
            }
          },
          "severity": "warning"
        }
      }
    ]
  }
}
```

## Use Cases

### 1. Dashboard Summary
Use `/api/pbs/backups` to get:
- Total backups per namespace
- Storage usage by namespace
- Guest distribution
- Collision warnings

### 2. Guest-Specific View
Use `/api/pbs/backups/guest/:type/:id` to:
- Show backup history for a specific VM/CT
- Track backup sizes over time
- Identify which PBS instances have backups

### 3. Collision Monitoring
Use `/api/pbs/collisions` to:
- Alert on VMID collisions
- Show severity (critical without namespaces, warning with)
- List affected guests and sources

### 4. Namespace Analysis
The backup data is pre-grouped by namespace, making it easy to:
- Show namespace utilization
- Compare backup distribution
- Identify namespace migration candidates

## Benefits

1. **Performance**: Server-side processing reduces client load
2. **Flexibility**: Query specific data without loading everything
3. **Scalability**: Easy to add filters, pagination, search
4. **Integration**: Clean REST API for external tools
5. **Real-time**: Always shows current PBS state

## Future Enhancements

1. **Filtering**:
   - Date range filters
   - Size filters
   - Verification status filters

2. **Search**:
   - Search by guest name
   - Search by comment/description

3. **Analytics**:
   - Growth trends
   - Backup success rates
   - Storage predictions

4. **Actions**:
   - Trigger verification
   - Initiate restore
   - Delete old backups