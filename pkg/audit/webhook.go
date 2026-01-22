package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	webhookQueueSize   = 1000
	webhookMaxRetries  = 3
	webhookTimeout     = 30 * time.Second
	webhookWorkerCount = 3
)

var webhookBackoff = []time.Duration{1 * time.Second, 5 * time.Second, 30 * time.Second}
var resolveWebhookIPs = net.DefaultResolver.LookupIPAddr

// WebhookDelivery handles async webhook delivery with retries.
type WebhookDelivery struct {
	mu       sync.RWMutex
	urls     []string
	client   *http.Client
	queue    chan Event
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// WebhookPayload is the JSON payload sent to webhooks.
type WebhookPayload struct {
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Data      Event     `json:"data"`
}

// NewWebhookDelivery creates a new webhook delivery worker.
func NewWebhookDelivery(urls []string) *WebhookDelivery {
	return &WebhookDelivery{
		urls: urls,
		client: &http.Client{
			Timeout: webhookTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  true,
				MaxConnsPerHost:     5,
				MaxIdleConnsPerHost: 2,
			},
		},
		queue:    make(chan Event, webhookQueueSize),
		stopChan: make(chan struct{}),
	}
}

// Start begins the delivery worker goroutines.
func (w *WebhookDelivery) Start() {
	for i := 0; i < webhookWorkerCount; i++ {
		w.wg.Add(1)
		go w.worker(i)
	}
	log.Debug().Int("workers", webhookWorkerCount).Msg("Audit webhook delivery started")
}

// Stop gracefully stops the delivery workers.
func (w *WebhookDelivery) Stop() {
	close(w.stopChan)
	w.wg.Wait()
	log.Debug().Msg("Audit webhook delivery stopped")
}

// Enqueue adds an event to the delivery queue.
// Non-blocking - drops events if queue is full.
func (w *WebhookDelivery) Enqueue(event Event) {
	select {
	case w.queue <- event:
		// Enqueued successfully
	default:
		log.Warn().
			Str("event_id", event.ID).
			Str("event_type", event.EventType).
			Msg("Audit webhook queue full, dropping event")
	}
}

// UpdateURLs updates the webhook URLs.
func (w *WebhookDelivery) UpdateURLs(urls []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.urls = urls
}

// GetURLs returns the current webhook URLs.
func (w *WebhookDelivery) GetURLs() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]string, len(w.urls))
	copy(result, w.urls)
	return result
}

// worker processes events from the queue.
func (w *WebhookDelivery) worker(id int) {
	defer w.wg.Done()

	for {
		select {
		case <-w.stopChan:
			// Drain remaining events on shutdown (with timeout)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			w.drainQueue(ctx)
			cancel()
			return
		case event := <-w.queue:
			w.deliverToAll(event)
		}
	}
}

// drainQueue processes remaining events during shutdown.
func (w *WebhookDelivery) drainQueue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			remaining := len(w.queue)
			if remaining > 0 {
				log.Warn().Int("remaining", remaining).Msg("Audit webhook shutdown timeout, dropping remaining events")
			}
			return
		case event := <-w.queue:
			w.deliverToAll(event)
		default:
			return
		}
	}
}

// deliverToAll sends an event to all configured webhooks.
func (w *WebhookDelivery) deliverToAll(event Event) {
	w.mu.RLock()
	urls := make([]string, len(w.urls))
	copy(urls, w.urls)
	w.mu.RUnlock()

	for _, url := range urls {
		if err := w.deliverWithRetry(url, event); err != nil {
			log.Error().
				Err(err).
				Str("url", url).
				Str("event_id", event.ID).
				Msg("Failed to deliver audit webhook after retries")
		}
	}
}

// deliverWithRetry attempts to deliver an event with exponential backoff.
func (w *WebhookDelivery) deliverWithRetry(url string, event Event) error {
	var lastErr error

	for attempt := 0; attempt <= webhookMaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			backoffIdx := attempt - 1
			if backoffIdx >= len(webhookBackoff) {
				backoffIdx = len(webhookBackoff) - 1
			}
			time.Sleep(webhookBackoff[backoffIdx])
		}

		err := w.deliver(url, event)
		if err == nil {
			if attempt > 0 {
				log.Debug().
					Str("url", url).
					Str("event_id", event.ID).
					Int("attempt", attempt+1).
					Msg("Audit webhook delivered after retry")
			}
			return nil
		}

		lastErr = err
		log.Debug().
			Err(err).
			Str("url", url).
			Str("event_id", event.ID).
			Int("attempt", attempt+1).
			Int("maxAttempts", webhookMaxRetries+1).
			Msg("Audit webhook delivery attempt failed")
	}

	return lastErr
}

// deliver sends a single event to a webhook URL.
func (w *WebhookDelivery) deliver(url string, event Event) error {
	payload := WebhookPayload{
		Event:     "audit." + event.EventType,
		Timestamp: event.Timestamp,
		Data:      event,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), webhookTimeout)
	defer cancel()

	if err := validateWebhookURL(ctx, url); err != nil {
		return fmt.Errorf("webhook URL blocked: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Pulse-Audit-Webhook/1.0")
	req.Header.Set("X-Pulse-Event", event.EventType)
	req.Header.Set("X-Pulse-Event-ID", event.ID)

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	// Consider 2xx as success
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func validateWebhookURL(ctx context.Context, rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https")
	}

	if parsed.Host == "" {
		return fmt.Errorf("URL must have a host")
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL must have a hostname")
	}

	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return fmt.Errorf("localhost URLs are not allowed")
	}

	if ip := net.ParseIP(hostname); ip != nil {
		if isPrivateOrReservedIP(ip) {
			return fmt.Errorf("private or reserved IP addresses are not allowed")
		}
		return nil
	}

	lowerHost := strings.ToLower(hostname)
	blockedPatterns := []string{
		"metadata.google",
		"169.254.169.254",
		"metadata.azure",
		"internal",
		".local",
		".localhost",
	}
	for _, pattern := range blockedPatterns {
		if strings.Contains(lowerHost, pattern) {
			return fmt.Errorf("internal hostnames are not allowed")
		}
	}

	if ctx == nil {
		ctx = context.Background()
	}
	resolveCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	addrs, err := resolveWebhookIPs(resolveCtx, hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname")
	}
	if len(addrs) == 0 {
		return fmt.Errorf("hostname did not resolve")
	}
	for _, addr := range addrs {
		if isPrivateOrReservedIP(addr.IP) {
			return fmt.Errorf("hostname resolves to private or reserved IP addresses")
		}
	}

	return nil
}

func isPrivateOrReservedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
		if ip4[0] == 0 {
			return true
		}
	}

	return false
}

// QueueLength returns the current number of events in the queue.
func (w *WebhookDelivery) QueueLength() int {
	return len(w.queue)
}
