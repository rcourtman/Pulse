//go:build windows

package hostagent

import "os"

func validateCustomSensorFileOwner(_ os.FileInfo, _ string) error {
	return nil
}
