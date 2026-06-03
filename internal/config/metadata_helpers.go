package config

// metadataFileLoader provides a generic way to load metadata from disk
type metadataFileLoader interface {
	LoadFromDisk() error
}
