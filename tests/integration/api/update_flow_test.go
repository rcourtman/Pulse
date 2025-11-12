package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

type updateInfo struct {
	Available    bool   `json:"available"`
	Current      string `json:"currentVersion"`
	Latest       string `json:"latestVersion"`
	DownloadURL  string `json:"downloadUrl"`
	IsPrerelease bool   `json:"isPrerelease"`
	ReleaseNotes string `json:"releaseNotes"`
	ReleaseDate  string `json:"releaseDate"`
	Warning      string `json:"warning"`
}

type updatePlan struct {
	CanAutoUpdate bool `json:"canAutoUpdate"`
}

type updateStatus struct {
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	Message   string `json:"message"`
	Error     string `json:"error"`
	UpdatedAt string `json:"updatedAt"`
}

func TestUpdateFlowIntegration(t *testing.T) {
	baseURL := strings.TrimRight(os.Getenv("UPDATE_API_BASE_URL"), "/")
	if baseURL == "" {
		t.Skip("UPDATE_API_BASE_URL not set; skipping integration test")
	}

	username := getenvDefault("UPDATE_API_USERNAME", "admin")
	password := getenvDefault("UPDATE_API_PASSWORD", "admin")

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("failed to create cookie jar: %v", err)
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
		Jar:     jar,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
	}

	waitForHealth(t, client, baseURL, 2*time.Minute)
	login(t, client, baseURL, username, password)

	info := fetchUpdateInfo(t, client, baseURL)
	if !info.Available {
		t.Fatalf("expected update to be available, got %+v", info)
	}
	if info.DownloadURL == "" {
		t.Fatalf("update info missing download URL: %+v", info)
	}

	plan := fetchUpdatePlan(t, client, baseURL, info.Latest)
	if !plan.CanAutoUpdate {
		t.Fatalf("expected plan to allow auto update: %+v", plan)
	}

	applyUpdate(t, client, baseURL, info.DownloadURL)
	waitForCompletion(t, client, baseURL, 2*time.Minute)
}

func waitForHealth(t *testing.T, client *http.Client, baseURL string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		resp, err := client.Get(baseURL + "/api/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		if time.Now().After(deadline) {
			t.Fatalf("health check failed: %v", err)
		}
		time.Sleep(2 * time.Second)
	}
}

func login(t *testing.T, client *http.Client, baseURL, username, password string) {
	t.Helper()
	payload := map[string]string{
		"username": username,
		"password": password,
	}
	resp := doJSONRequest(t, client, "POST", baseURL+"/api/login", payload)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login failed with status %s", resp.Status)
	}
}

func fetchUpdateInfo(t *testing.T, client *http.Client, baseURL string) updateInfo {
	t.Helper()
	resp := doRequest(t, client, "GET", baseURL+"/api/updates/check", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update check failed with status %s", resp.Status)
	}
	var info updateInfo
	decodeJSON(t, resp, &info)
	return info
}

func fetchUpdatePlan(t *testing.T, client *http.Client, baseURL, version string) updatePlan {
	t.Helper()
	endpoint := fmt.Sprintf("%s/api/updates/plan?version=%s", baseURL, url.QueryEscape(version))
	resp := doRequest(t, client, "GET", endpoint, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update plan fetch failed with status %s", resp.Status)
	}
	var plan updatePlan
	decodeJSON(t, resp, &plan)
	return plan
}

func applyUpdate(t *testing.T, client *http.Client, baseURL, downloadURL string) {
	t.Helper()
	payload := map[string]string{"downloadUrl": downloadURL}
	resp := doJSONRequest(t, client, "POST", baseURL+"/api/updates/apply", payload)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("apply update failed with status %s", resp.Status)
	}
}

func waitForCompletion(t *testing.T, client *http.Client, baseURL string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	seenStages := make(map[string]struct{})
	for {
		resp := doRequest(t, client, "GET", baseURL+"/api/updates/status", nil)
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.Fatalf("status endpoint returned %s", resp.Status)
		}
		var status updateStatus
		decodeJSON(t, resp, &status)
		resp.Body.Close()

		seenStages[status.Status] = struct{}{}
		if status.Error != "" {
			t.Fatalf("update failed: %s (%s)", status.Error, status.Message)
		}
		if status.Status == "completed" {
			if _, ok := seenStages["downloading"]; !ok {
				t.Fatalf("expected downloading stage, got %+v", seenStages)
			}
			if _, ok := seenStages["applying"]; !ok {
				t.Fatalf("expected applying stage, got %+v", seenStages)
			}
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("update did not complete within %s (last status: %+v)", timeout, status)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func doJSONRequest(t *testing.T, client *http.Client, method, endpoint string, payload any) *http.Response {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	return doRequest(t, client, method, endpoint, bytes.NewReader(data), "application/json")
}

func doRequest(t *testing.T, client *http.Client, method, endpoint string, body io.Reader, contentType ...string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	if len(contentType) > 0 && contentType[0] != "" {
		req.Header.Set("Content-Type", contentType[0])
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request %s %s failed: %v", method, endpoint, err)
	}
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response, dest any) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		t.Fatalf("failed to decode JSON from %s: %v", resp.Request.URL, err)
	}
}

func getenvDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
