package api

import (
	"fmt"
	"reflect"
	"time"
)

const agentReportTimeZoneLimitSeconds = 24 * 60 * 60

var agentReportTimeType = reflect.TypeOf(time.Time{})

// validateAgentReportTimes rejects a decoded agent report that holds a time
// the server cannot encode again. encoding/json decodes RFC 3339 times that
// time.Time.MarshalJSON refuses: 9999-12-31T23:59:59-23:59 is year 10000 once
// a consumer normalizes it with UTC(), and a +24:00 zone offset cannot be
// encoded at all. One such value fails the whole state, resource, alert or
// recovery document it is copied into, so it must not get past the decoder.
func validateAgentReportTimes(report any) error {
	return validateAgentReportTimesIn(reflect.ValueOf(report))
}

func validateAgentReportTimesIn(v reflect.Value) error {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return nil
		}
		return validateAgentReportTimesIn(v.Elem())
	case reflect.Struct:
		if v.Type() == agentReportTimeType {
			// Everything but a map value is addressable, and reading through
			// the address spares an allocation per time on large inventories.
			if v.CanAddr() {
				return validateAgentReportTime(*v.Addr().Interface().(*time.Time))
			}
			return validateAgentReportTime(v.Interface().(time.Time))
		}
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			// encoding/json does not decode into unexported fields.
			if !field.CanInterface() || !agentReportKindCanHoldTime(field.Kind()) {
				continue
			}
			if err := validateAgentReportTimesIn(field); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		if !agentReportKindCanHoldTime(v.Type().Elem().Kind()) {
			return nil
		}
		for i := 0; i < v.Len(); i++ {
			if err := validateAgentReportTimesIn(v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		if !agentReportKindCanHoldTime(v.Type().Elem().Kind()) {
			return nil
		}
		for iter := v.MapRange(); iter.Next(); {
			if err := validateAgentReportTimesIn(iter.Value()); err != nil {
				return err
			}
		}
	}
	return nil
}

// agentReportKindCanHoldTime keeps the walk out of scalars, label maps, string
// lists and byte slices. encoding/json never decodes a time.Time into an
// interface value, so those are not followed either.
func agentReportKindCanHoldTime(kind reflect.Kind) bool {
	switch kind {
	case reflect.Pointer, reflect.Struct, reflect.Slice, reflect.Array, reflect.Map:
		return true
	default:
		return false
	}
}

// validateAgentReportTime applies the limits of time.Time.MarshalJSON, zone
// hours 0..23 and years 0..9999, to the time as it was received and to the
// same instant in UTC, which is how consumers and the stores normalize report
// times.
func validateAgentReportTime(t time.Time) error {
	if _, offset := t.Zone(); offset <= -agentReportTimeZoneLimitSeconds || offset >= agentReportTimeZoneLimitSeconds {
		return fmt.Errorf("report time %s has a zone offset of 24 hours or more", t.Format(time.RFC3339Nano))
	}
	if year, utcYear := t.Year(), t.UTC().Year(); year < 0 || year > 9999 || utcYear < 0 || utcYear > 9999 {
		return fmt.Errorf("report time %s is outside years 0 to 9999 as received or in UTC", t.Format(time.RFC3339Nano))
	}
	return nil
}
