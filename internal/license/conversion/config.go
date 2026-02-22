package conversion

import pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"

type CollectionConfigSnapshot = pkglicensing.CollectionConfigSnapshot
type CollectionConfig = pkglicensing.CollectionConfig

// NewCollectionConfig returns a default runtime config with collection enabled.
func NewCollectionConfig() *CollectionConfig {
	return pkglicensing.NewCollectionConfig()
}
