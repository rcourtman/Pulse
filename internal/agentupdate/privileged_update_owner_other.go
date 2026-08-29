//go:build !unix

package agentupdate

import "os"

func collectorQuarantineOwnedByCurrentUID(os.FileInfo) bool { return false }
