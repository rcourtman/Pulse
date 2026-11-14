package lifecycle

import (
	"fmt"
	"os"
)

func openAndLock(string) (*os.File, error) {
	return nil, fmt.Errorf("hosted runtime lifecycle requires a Unix Docker host")
}
