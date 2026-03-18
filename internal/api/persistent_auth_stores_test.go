package api

import (
	"testing"
	"time"
)

func resetPersistentAuthStoresForTests() {
	resetSessionStoreForTests()
	resetCSRFStoreForTests()
	resetRecoveryStoreForTests()
}

func TestPersistentAuthStoresRequireExplicitInitialization(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)

	assertPanics := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Fatalf("%s should panic before explicit initialization", name)
			}
		}()
		fn()
	}

	assertPanics("session store", func() { _ = GetSessionStore() })
	assertPanics("csrf store", func() { _ = GetCSRFStore() })
	assertPanics("recovery token store", func() { _ = GetRecoveryTokenStore() })
}

func TestPersistentAuthStoresReconfigureDataPath(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)

	dirOne := t.TempDir()
	dirTwo := t.TempDir()

	InitSessionStore(dirOne)
	GetSessionStore().CreateSession("session-one", time.Hour, "agent", "127.0.0.1", "alice")
	InitSessionStore(dirTwo)
	if GetSessionStore().GetSession("session-one") != nil {
		t.Fatal("reconfigured session store should not retain sessions from the previous data path")
	}

	InitCSRFStore(dirOne)
	csrfToken := GetCSRFStore().GenerateCSRFToken("session-one")
	InitCSRFStore(dirTwo)
	if GetCSRFStore().ValidateCSRFToken("session-one", csrfToken) {
		t.Fatal("reconfigured csrf store should not retain tokens from the previous data path")
	}

	InitRecoveryTokenStore(dirOne)
	recoveryToken, err := GetRecoveryTokenStore().GenerateRecoveryToken(time.Hour)
	if err != nil {
		t.Fatalf("generate recovery token: %v", err)
	}
	InitRecoveryTokenStore(dirTwo)
	if GetRecoveryTokenStore().IsRecoveryTokenValidConstantTime(recoveryToken) {
		t.Fatal("reconfigured recovery token store should not retain tokens from the previous data path")
	}
}
