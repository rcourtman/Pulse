package hostmetrics

import (
    "context"
    "encoding/json"
    "testing"
)

func TestCollectDiskIO(t *testing.T) {
    ctx := context.Background()
    
    snapshot, err := Collect(ctx, nil)
    if err != nil {
        t.Fatalf("Collect failed: %v", err)
    }
    
    t.Logf("DiskIO count: %d", len(snapshot.DiskIO))
    
    if len(snapshot.DiskIO) == 0 {
        t.Error("Expected disk IO data but got none")
    }
    
    data, _ := json.MarshalIndent(snapshot.DiskIO, "", "  ")
    t.Logf("DiskIO data:\n%s", string(data))
}
