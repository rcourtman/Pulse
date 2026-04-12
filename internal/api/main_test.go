package api

import (
	"os"
	"strconv"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("PULSE_TEST_BCRYPT_COST", strconv.Itoa(bcrypt.MinCost))
	_ = os.Setenv("PULSE_UPDATE_SERVER", "http://127.0.0.1:1")
	allowLoopbackSSOFetch = true
	log.Logger = zerolog.Nop()
	os.Exit(m.Run())
}
