//go:build !darwin && !linux && !windows

package authstore

type unavailableKeychainStore struct{}

func defaultKeychainStore(_ string) SecretStore {
	return &unavailableKeychainStore{}
}

func (k *unavailableKeychainStore) Backend() string {
	return BackendKeychain
}

func (k *unavailableKeychainStore) Available() bool {
	return false
}

func (k *unavailableKeychainStore) Set(_, _ string) error {
	return ErrKeychainUnavailable
}

func (k *unavailableKeychainStore) Get(_ string) (string, error) {
	return "", ErrKeychainUnavailable
}

func (k *unavailableKeychainStore) Delete(_ string) error {
	return nil
}
