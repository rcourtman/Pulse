package license

import pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"

const (
	LicenseFileName       = pkglicensing.LicenseFileName
	PersistentKeyFileName = pkglicensing.PersistentKeyFileName
)

type Persistence = pkglicensing.Persistence
type PersistedLicense = pkglicensing.PersistedLicense

func NewPersistence(configDir string) (*Persistence, error) {
	return pkglicensing.NewPersistence(configDir)
}
