package api

import (
	"encoding/json"
	"net/http"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func handleTestInstallationStatus(w http.ResponseWriter, r *http.Request, licenseVersion int64) bool {
	if r.URL.Path != "/v1/grants/status" {
		return false
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return true
	}
	_ = json.NewEncoder(w).Encode(pkglicensing.InstallationStatusResponse{
		LicenseVersion: licenseVersion,
		StatusPolicy: pkglicensing.StatusHints{
			IntervalSeconds: 300,
			JitterPercent:   20,
		},
	})
	return true
}
