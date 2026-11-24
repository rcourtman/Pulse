package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var errAlertConfigUpdateNotObject = errors.New("alert configuration update must be a JSON object")

// Limits on the lists of a client's update whose entries are objects. The
// request is limited in bytes, but "{}" is a whole entry there and a complete
// rule, filter or escalation level once decoded: 1 MB of empty custom rules
// took about 1 GB to apply and was saved as a 74 MB alerts.json that every
// later read of the configuration copied. Lists written by hand stay far
// below these. Overrides grow with the fleet and are left to the size limit
// of the request.
const (
	maxCustomRules       = 500
	maxCustomRuleFilters = 100
	maxEscalationLevels  = 100
)

// customRuleCount stands in for AlertConfig.CustomRules to count the rules of
// an update before any of them is decoded, because decoding is what allocates
// them. encoding/json matches the key to it the way it matches the key to the
// field, and hands it every occurrence of that key, so a list the body
// repeats or spells in another case is counted as well.
type customRuleCount struct{}

func (*customRuleCount) UnmarshalJSON(data []byte) error {
	// An entry decodes to nothing, whatever it holds. A value that is not a
	// list has no entries and is left for the decode of the update to refuse.
	var rules []struct{}
	_ = json.Unmarshal(data, &rules)
	if len(rules) > maxCustomRules {
		return fmt.Errorf("alert configuration update carries %d custom rules, at most %d are supported", len(rules), maxCustomRules)
	}
	return nil
}

// checkAlertConfigUpdateLists refuses a decoded update whose custom rule
// filters or escalation levels outgrow their limits.
func checkAlertConfigUpdateLists(update AlertConfig) error {
	for _, rule := range update.CustomRules {
		if filters := len(rule.FilterConditions.Filters); filters > maxCustomRuleFilters {
			return fmt.Errorf("alert configuration update carries a custom rule with %d filters, at most %d are supported", filters, maxCustomRuleFilters)
		}
	}
	if levels := len(update.Schedule.Escalation.Levels); levels > maxEscalationLevels {
		return fmt.Errorf("alert configuration update carries %d escalation levels, at most %d are supported", levels, maxEscalationLevels)
	}
	return nil
}

// ApplyAlertConfigUpdate decodes a client's alert configuration update onto
// the stored configuration. A top-level key the client sent replaces that
// setting whole; a key it left out keeps the stored value.
//
// Clients only send what they know about. The settings page has no control
// for flapping detection, alert TTL cleanup or custom rules, and an older
// browser bundle or API script predates any field added since. Decoding the
// body into a zero AlertConfig used to turn every one of those settings off
// on each save.
//
// The body is decoded on its own, so the keys it carries mean exactly what
// encoding/json makes of them. Each setting it left out is copied from a JSON
// round trip of stored, so the result shares no maps or slices with it.
//
// An update whose custom rules, rule filters or escalation levels outgrow
// their limits is refused.
func ApplyAlertConfigUpdate(stored AlertConfig, update []byte) (AlertConfig, error) {
	var sent map[string]json.RawMessage
	if err := json.Unmarshal(update, &sent); err != nil {
		return AlertConfig{}, err
	}
	if sent == nil {
		return AlertConfig{}, errAlertConfigUpdateNotObject
	}
	var counted struct {
		CustomRules customRuleCount `json:"customRules"`
	}
	if err := json.Unmarshal(update, &counted); err != nil {
		return AlertConfig{}, err
	}
	var result AlertConfig
	if err := json.Unmarshal(update, &result); err != nil {
		return AlertConfig{}, err
	}
	if err := checkAlertConfigUpdateLists(result); err != nil {
		return AlertConfig{}, err
	}

	kept, err := CloneAlertConfig(stored)
	if err != nil {
		return AlertConfig{}, err
	}

	resultValue := reflect.ValueOf(&result).Elem()
	keptValue := reflect.ValueOf(kept)
	for i := 0; i < resultValue.NumField(); i++ {
		if !alertConfigKeySent(sent, alertConfigJSONKey(resultValue.Type().Field(i))) {
			resultValue.Field(i).Set(keptValue.Field(i))
		}
	}
	return result, nil
}

// CloneAlertConfig returns a copy of config that shares no maps, slices or
// pointers with it, by the same JSON round trip config takes through
// alerts.json.
func CloneAlertConfig(config AlertConfig) (AlertConfig, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return AlertConfig{}, fmt.Errorf("encode alert config: %w", err)
	}
	var clone AlertConfig
	if err := json.Unmarshal(data, &clone); err != nil {
		return AlertConfig{}, fmt.Errorf("decode alert config: %w", err)
	}
	return clone, nil
}

// alertConfigJSONKey returns the key encoding/json reads a field from.
func alertConfigJSONKey(field reflect.StructField) string {
	name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
	if name == "" {
		return field.Name
	}
	return name
}

// alertConfigKeySent matches keys the way encoding/json matches them to
// fields: exactly, or else case-insensitively.
func alertConfigKeySent(sent map[string]json.RawMessage, key string) bool {
	if _, ok := sent[key]; ok {
		return true
	}
	for sentKey := range sent {
		if strings.EqualFold(sentKey, key) {
			return true
		}
	}
	return false
}
