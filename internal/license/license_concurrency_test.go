package license

import (
	"crypto/ed25519"
	"sync"
	"testing"
	"time"
)

func TestSetPublicKeyConcurrentWithValidateLicense(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")
	SetPublicKey(nil)
	t.Cleanup(func() {
		SetPublicKey(nil)
	})

	licenseKey, err := GenerateLicenseForTesting("race@example.com", TierPro, time.Hour)
	if err != nil {
		t.Fatalf("GenerateLicenseForTesting: %v", err)
	}

	const goroutines = 8
	const iterations = 400

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for worker := 0; worker < goroutines; worker++ {
		seed := byte(worker + 1)

		go func(seed byte) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if i%2 == 0 {
					SetPublicKey(nil)
					continue
				}

				key := make(ed25519.PublicKey, ed25519.PublicKeySize)
				key[0] = seed
				key[ed25519.PublicKeySize-1] = byte(i)
				SetPublicKey(key)

				// Mutate the caller-owned key after SetPublicKey returns; service state
				// must remain isolated from this memory.
				key[0] ^= 0xFF
			}
		}(seed)

		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_, _ = ValidateLicense(licenseKey)
			}
		}()
	}

	wg.Wait()
}
