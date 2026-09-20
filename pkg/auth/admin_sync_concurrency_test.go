package auth

import (
	"context"
	"sync"
	"testing"
)

func TestRBACAdminIdentityConcurrentUpdates(t *testing.T) {
	manager, err := NewFileManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := NewRBACAuthorizer(manager)
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Go(func() {
			for i := 0; i < 100; i++ {
				a.SetAdminUser("configured-admin")
				a.Authorize(WithUser(context.Background(), "configured-admin"), ActionAdmin, ResourceUsers)
				if allowed, err := a.Authorize(WithUser(context.Background(), "outsider"), ActionAdmin, ResourceUsers); allowed || err != nil {
					t.Errorf("outsider allowed=%v err=%v", allowed, err)
				}
				a.SetAdminUser("")
			}
		})
	}
	wg.Wait()
	a.SetAdminUser("")
	if allowed, err := a.Authorize(WithUser(context.Background(), "configured-admin"), ActionAdmin, ResourceUsers); allowed || err != nil {
		t.Fatalf("cleared admin allowed=%v err=%v", allowed, err)
	}
}
