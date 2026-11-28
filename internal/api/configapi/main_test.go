package configapi

import (
	"os"
	"strconv"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	if _, ok := os.LookupEnv("PULSE_TEST_BCRYPT_COST"); !ok {
		_ = os.Setenv("PULSE_TEST_BCRYPT_COST", strconv.Itoa(bcrypt.MinCost))
	}
	if _, ok := os.LookupEnv("PULSE_TEST_EXPORT_PBKDF2_ITERATIONS"); !ok {
		_ = os.Setenv("PULSE_TEST_EXPORT_PBKDF2_ITERATIONS", "1000")
	}
	os.Exit(m.Run())
}
