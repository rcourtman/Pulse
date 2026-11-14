package unifiedresources

// ReadRefresher is a read state whose owner can publish the state it has
// already collected into it before a reader acts on it. The monitor refreshes
// its store at ingest boundaries and on frontend and REST reads, but not after
// every poll, so a headless reader holding the store can see values older than
// the alerts it is reacting to.
type ReadRefresher interface {
	RefreshForRead()
}

// RefreshForRead publishes the owner's latest collected state into rs when rs
// supports it, and does nothing otherwise. Call it once before acting on a
// read, never per lookup: the refresh rebuilds the whole registry.
func RefreshForRead(rs ReadState) {
	if refresher, ok := rs.(ReadRefresher); ok && refresher != nil {
		refresher.RefreshForRead()
	}
}
