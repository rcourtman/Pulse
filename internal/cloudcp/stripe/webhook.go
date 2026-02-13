package stripe

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpmetrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
	stripelib "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
)

const webhookBodyLimit = 1024 * 1024 // 1 MiB

// WebhookHandler handles incoming Stripe webhook events.
type WebhookHandler struct {
	secret      string
	provisioner *Provisioner
}

type webhookErrorResponse struct {
	Error string `json:"error"`
}

type webhookReceivedResponse struct {
	Received bool `json:"received"`
}

// NewWebhookHandler creates a Stripe webhook HTTP handler.
func NewWebhookHandler(secret string, provisioner *Provisioner) *WebhookHandler {
	return &WebhookHandler{
		secret:      secret,
		provisioner: provisioner,
	}
}

// ServeHTTP verifies the Stripe signature and dispatches the event.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	eventType := "unknown"
	status := http.StatusOK
	defer func() {
		cpmetrics.WebhookRequestsTotal.WithLabelValues(eventType, strconv.Itoa(status)).Inc()
		cpmetrics.WebhookDuration.WithLabelValues(eventType).Observe(time.Since(start).Seconds())
	}()

	if r.Method != http.MethodPost {
		status = http.StatusMethodNotAllowed
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}
	if strings.TrimSpace(h.secret) == "" {
		status = http.StatusServiceUnavailable
		writeJSON(w, http.StatusServiceUnavailable, webhookErrorResponse{Error: "webhook secret not configured"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, webhookBodyLimit)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		status = http.StatusBadRequest
		writeJSON(w, http.StatusBadRequest, webhookErrorResponse{Error: "failed to read request body"})
		return
	}

	sigHeader := r.Header.Get("Stripe-Signature")
	if strings.TrimSpace(sigHeader) == "" {
		status = http.StatusBadRequest
		writeJSON(w, http.StatusBadRequest, webhookErrorResponse{Error: "missing Stripe signature"})
		return
	}

	event, err := webhook.ConstructEventWithOptions(payload, sigHeader, h.secret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		status = http.StatusBadRequest
		writeJSON(w, http.StatusBadRequest, webhookErrorResponse{Error: "invalid Stripe signature"})
		return
	}
	eventType = string(event.Type)

	if err := h.handleEvent(r, &event); err != nil {
		log.Error().Err(err).
			Str("event_id", event.ID).
			Str("type", string(event.Type)).
			Msg("Stripe webhook processing failed")
		status = http.StatusInternalServerError
		writeJSON(w, http.StatusInternalServerError, webhookErrorResponse{Error: "processing failed"})
		return
	}

	status = http.StatusOK
	writeJSON(w, http.StatusOK, webhookReceivedResponse{Received: true})
}

func (h *WebhookHandler) handleEvent(r *http.Request, event *stripelib.Event) error {
	switch event.Type {
	case "checkout.session.completed":
		var session CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			return fmt.Errorf("decode checkout.session: %w", err)
		}
		return h.provisioner.HandleCheckout(r.Context(), session)

	case "customer.subscription.updated":
		var sub Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return fmt.Errorf("decode subscription: %w", err)
		}
		return h.routeSubscriptionUpdated(r, sub)

	case "customer.subscription.deleted":
		var sub Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return fmt.Errorf("decode subscription: %w", err)
		}
		return h.routeSubscriptionDeleted(r, sub)

	default:
		log.Info().
			Str("type", string(event.Type)).
			Str("event_id", event.ID).
			Msg("Stripe webhook ignored (unhandled type)")
		return nil
	}
}

func (h *WebhookHandler) routeSubscriptionUpdated(r *http.Request, sub Subscription) error {
	customerID := strings.TrimSpace(sub.Customer)
	if customerID != "" {
		sa, err := h.provisioner.registry.GetStripeAccountByCustomerID(customerID)
		if err != nil {
			return fmt.Errorf("lookup stripe account by customer: %w", err)
		}
		if sa != nil {
			acct, err := h.provisioner.registry.GetAccount(sa.AccountID)
			if err != nil {
				return fmt.Errorf("lookup account: %w", err)
			}
			if acct != nil && acct.Kind == registry.AccountKindMSP {
				return h.provisioner.HandleMSPSubscriptionUpdated(r.Context(), sub)
			}
		}
	}
	return h.provisioner.HandleSubscriptionUpdated(r.Context(), sub)
}

func (h *WebhookHandler) routeSubscriptionDeleted(r *http.Request, sub Subscription) error {
	customerID := strings.TrimSpace(sub.Customer)
	if customerID != "" {
		sa, err := h.provisioner.registry.GetStripeAccountByCustomerID(customerID)
		if err != nil {
			return fmt.Errorf("lookup stripe account by customer: %w", err)
		}
		if sa != nil {
			acct, err := h.provisioner.registry.GetAccount(sa.AccountID)
			if err != nil {
				return fmt.Errorf("lookup account: %w", err)
			}
			if acct != nil && acct.Kind == registry.AccountKindMSP {
				return h.provisioner.HandleMSPSubscriptionDeleted(r.Context(), sub)
			}
		}
	}
	return h.provisioner.HandleSubscriptionDeleted(r.Context(), sub)
}

// CheckoutSession is a minimal representation of a Stripe checkout.session event.
type CheckoutSession struct {
	ID              string `json:"id"`
	Mode            string `json:"mode"`
	Customer        string `json:"customer"`
	Subscription    string `json:"subscription"`
	CustomerEmail   string `json:"customer_email"`
	CustomerDetails struct {
		Email string `json:"email"`
	} `json:"customer_details"`
	Metadata map[string]string `json:"metadata"`
}

// Subscription is a minimal representation of a Stripe subscription event.
type Subscription struct {
	ID                string `json:"id"`
	Customer          string `json:"customer"`
	Status            string `json:"status"`
	CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
	Items             struct {
		Data []struct {
			Price struct {
				ID       string            `json:"id"`
				Metadata map[string]string `json:"metadata"`
			} `json:"price"`
		} `json:"data"`
	} `json:"items"`
	Metadata map[string]string `json:"metadata"`
}

// FirstPriceID returns the price ID from the first subscription item.
func (s *Subscription) FirstPriceID() string {
	for _, item := range s.Items.Data {
		if priceID := strings.TrimSpace(item.Price.ID); priceID != "" {
			return priceID
		}
	}
	return ""
}

func writeJSON[T any](w http.ResponseWriter, status int, v T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error().Err(err).Int("status", status).Msg("cloudcp.stripe: encode webhook response")
	}
}
