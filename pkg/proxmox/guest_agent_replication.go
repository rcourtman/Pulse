package proxmox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
	"unicode/utf8"
)

// PVE's guest_fs_freeze allows a freeze one hour in total, and a replication run
// thaws as soon as its snapshots exist. Past this age a live job is transferring,
// which can take hours and must not hold guest readings.
const guestReplicationFreezeWindow = time.Hour

// A replication run freezes the guest through QGA while holding only PVE's local
// migration file lock. It sets no config lock, so verifyGuestAgentUnlocked cannot
// see it, and PVE only warns if the thaw that follows fails.
//
// The job's status read is protected, so it needs a pvedaemon worker from the
// same pool guest commands wait on. The inventory the monitor already polls limits
// it to guests that have a job. Until that inventory first succeeds, or for a job
// created since the last one, the guest is not checked.
func (c *Client) verifyGuestAgentNotReplicating(ctx context.Context, node string, vmid int) error {
	if !guestHasReplicationJob(c.baseURL, vmid) {
		return nil
	}
	body, answered, err := c.readGuestReplicationStatus(ctx, node, vmid)
	if err != nil {
		return &guestAgentDeferredError{reason: "lock-unverified", cause: err}
	}
	if !answered {
		return nil
	}
	freezing, valid := guestReplicationMayBeFreezing(body, time.Now())
	if !valid {
		return &guestAgentDeferredError{reason: "lock-unverified"}
	}
	if freezing {
		return &guestAgentDeferredError{reason: "replication-active"}
	}
	return nil
}

// Each inventory is filtered by its token's VM.Audit, so leaving a guest out does
// not show its job has gone, and this only ever adds. A guest whose job was removed
// costs a status read that reports nothing running. Cluster endpoints share one
// view because any of them can carry the guest command.
func recordGuestReplicationJobs(endpoint string, jobs []ReplicationJob) {
	key, valid := guestAgentEndpointKey(endpoint)
	if !valid {
		return
	}
	guestAgentGuards.Lock()
	defer guestAgentGuards.Unlock()
	endpoints := guestAgentGuards.aliases[key]
	if len(endpoints) == 0 {
		endpoints = []string{key}
	}
	for _, job := range jobs {
		if job.GuestID <= 0 {
			continue
		}
		for _, endpoint := range endpoints {
			guests := guestAgentGuards.replicationGuests[endpoint]
			if guests == nil {
				guests = make(map[int]bool)
				guestAgentGuards.replicationGuests[endpoint] = guests
			}
			guests[job.GuestID] = true
		}
	}
}

func guestHasReplicationJob(endpoint string, vmid int) bool {
	key, valid := guestAgentEndpointKey(endpoint)
	if !valid {
		return false
	}
	guestAgentGuards.Lock()
	defer guestAgentGuards.Unlock()
	return guestAgentGuards.replicationGuests[key][vmid]
}

// A complete answer of 404 or 501 means an unknown route. A complete 500 is PVE's
// job_status failing, as it does without quorum; get_next_job calls it too, so no
// new job can start, and deferring would only cost readings during that outage.
// Every other failure leaves the job's state unknown and stays an error.
func (c *Client) readGuestReplicationStatus(ctx context.Context, node string, vmid int) ([]byte, bool, error) {
	readClient := *c.httpClient
	readClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	path := fmt.Sprintf("/nodes/%s/replication?guest=%d", node, vmid)
	resp, err := c.requestWithRetryUsingClient(ctx, http.MethodGet, path, nil, false, &readClient)
	if err != nil {
		var response *apiResponseError
		if errors.As(err, &response) {
			switch response.statusCode {
			case http.StatusNotFound, http.StatusNotImplemented, http.StatusInternalServerError:
				return nil, false, nil
			}
		}
		return nil, false, err
	}
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("replication status read returned status %d", resp.StatusCode)
	}
	if readErr != nil || closeErr != nil {
		return nil, false, errors.Join(readErr, closeErr)
	}
	return body, true, nil
}

// PVE includes pid only while the job's process is alive, and records last_try
// before the freeze. An unusable last_try can't show the freeze has passed.
func guestReplicationMayBeFreezing(body []byte, now time.Time) (bool, bool) {
	if !utf8.Valid(body) {
		return false, false
	}
	envelope, ok := guestAgentResponseFields(body, "data")
	if !ok {
		return false, false
	}
	var entries *[]json.RawMessage
	if json.Unmarshal(envelope["data"], &entries) != nil || entries == nil {
		return false, false
	}
	freezing := false
	for _, raw := range *entries {
		entry, ok := guestAgentResponseFields(raw)
		if !ok {
			return false, false
		}
		value, present := entry["pid"]
		if !present {
			continue
		}
		pid, ok := intFromAny(decodeRaw(value))
		if !ok {
			return false, false
		}
		if pid <= 0 {
			continue
		}
		lastTry, ok := intFromAny(decodeRaw(entry["last_try"]))
		if !ok || lastTry <= 0 || now.Sub(time.Unix(int64(lastTry), 0)) < guestReplicationFreezeWindow {
			freezing = true
		}
	}
	return freezing, true
}
