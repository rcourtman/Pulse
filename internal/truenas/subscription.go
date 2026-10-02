package truenas

import (
	"encoding/json"
	"errors"
	"fmt"
)

// A clean terminal event completes a log tail without leaving a subscription
// to cancel. It is not a sample for a telemetry stream.
var errRPCSubscriptionComplete = errors.New("truenas rpc subscription completed")

// rpcSubscriptionTermination recognises only the exact collection we subscribed
// to, including its arguments. Other subscriptions (even another interval for
// the same event source) must not terminate the current reader. middlewared's
// notification error is not a JSON-RPC response error: it uses errno/reason
// fields, and may contain private names and traces. Retain only its numeric
// errno and a fixed message, never that wire text or the collection arguments.
func rpcSubscriptionTermination(message trueNASRPCResponse, collection, method string) (bool, error) {
	if message.Method != "notify_unsubscribed" {
		return false, nil
	}
	var notification struct {
		Collection string          `json:"collection"`
		Error      json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(message.Params, &notification); err != nil {
		return false, fmt.Errorf("invalid truenas %s termination notification", method)
	}
	if notification.Collection != collection {
		return false, nil
	}
	if len(notification.Error) == 0 {
		return true, fmt.Errorf("invalid truenas %s termination notification", method)
	}
	if string(notification.Error) == "null" {
		return true, nil
	}
	var failure struct {
		Errno int `json:"error"`
	}
	if err := json.Unmarshal(notification.Error, &failure); err != nil {
		return true, fmt.Errorf("invalid truenas %s termination error", method)
	}
	return true, &RPCError{Method: method, Code: failure.Errno, Message: "subscription rejected"}
}

func rpcSubscriptionEndedBeforeData(method string) error {
	return &RPCError{Method: method, Message: "subscription ended before a sample"}
}
