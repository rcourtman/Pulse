package api

import (
	"os"
	"strconv"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("PULSE_TEST_BCRYPT_COST", strconv.Itoa(bcrypt.MinCost))
	_ = os.Setenv("PULSE_UPDATE_SERVER", "http://127.0.0.1:1")
	os.Exit(m.Run())
}
