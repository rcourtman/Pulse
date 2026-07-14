package qualification

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestComparePredicate(t *testing.T) {
	cases := []struct {
		name      string
		observed  any
		operator  string
		raw       json.RawMessage
		wantBool  bool
		wantErr   bool
		errSubstr string
	}{
		// len(raw) == 0 arm: required-value error.
		{
			name:      "empty raw returns required error",
			observed:  "anything",
			operator:  "eq",
			raw:       json.RawMessage(""),
			wantErr:   true,
			errSubstr: "predicate value is required",
		},
		{
			name:      "nil raw returns required error",
			observed:  5,
			operator:  "eq",
			raw:       nil,
			wantErr:   true,
			errSubstr: "predicate value is required",
		},
		// json.Unmarshal failure arm.
		{
			name:     "invalid json returns unmarshal error",
			observed: 1,
			operator: "eq",
			raw:      json.RawMessage("{bad"),
			wantErr:  true,
		},
		// eq operator: fmt.Sprint equality.
		{
			name:     "eq matching strings true",
			observed: "running",
			operator: "eq",
			raw:      json.RawMessage(`"running"`),
			wantBool: true,
		},
		{
			name:     "eq mismatching strings false",
			observed: "running",
			operator: "eq",
			raw:      json.RawMessage(`"stopped"`),
			wantBool: false,
		},
		{
			name:     "eq int and json number sprint equal true",
			observed: 5,
			operator: "eq",
			raw:      json.RawMessage(`5`),
			wantBool: true,
		},
		{
			name:     "eq float with decimal true",
			observed: 5.5,
			operator: "eq",
			raw:      json.RawMessage(`5.5`),
			wantBool: true,
		},
		{
			name:     "eq bool true",
			observed: true,
			operator: "eq",
			raw:      json.RawMessage(`true`),
			wantBool: true,
		},
		{
			name:     "eq nil observed and json null sprint equal true",
			observed: nil,
			operator: "eq",
			raw:      json.RawMessage(`null`),
			wantBool: true,
		},
		{
			name:     "eq int and numeric string coerce equal true",
			observed: 5,
			operator: "eq",
			raw:      json.RawMessage(`"5"`),
			wantBool: true,
		},
		// not_eq operator: fmt.Sprint inequality.
		{
			name:     "not_eq equal values false",
			observed: "running",
			operator: "not_eq",
			raw:      json.RawMessage(`"running"`),
			wantBool: false,
		},
		{
			name:     "not_eq different values true",
			observed: "running",
			operator: "not_eq",
			raw:      json.RawMessage(`"stopped"`),
			wantBool: true,
		},
		// numeric operators gte/lte/gt/lt.
		{
			name:     "gte greater true",
			observed: 10,
			operator: "gte",
			raw:      json.RawMessage(`5`),
			wantBool: true,
		},
		{
			name:     "gte equal boundary true",
			observed: 5,
			operator: "gte",
			raw:      json.RawMessage(`5`),
			wantBool: true,
		},
		{
			name:     "gte less false",
			observed: 1,
			operator: "gte",
			raw:      json.RawMessage(`5`),
			wantBool: false,
		},
		{
			name:     "lte less true",
			observed: 1,
			operator: "lte",
			raw:      json.RawMessage(`5`),
			wantBool: true,
		},
		{
			name:     "lte equal boundary true",
			observed: 5,
			operator: "lte",
			raw:      json.RawMessage(`5`),
			wantBool: true,
		},
		{
			name:     "lte greater false",
			observed: 10,
			operator: "lte",
			raw:      json.RawMessage(`5`),
			wantBool: false,
		},
		{
			name:     "gt greater true",
			observed: 10,
			operator: "gt",
			raw:      json.RawMessage(`5`),
			wantBool: true,
		},
		{
			name:     "gt equal boundary false",
			observed: 5,
			operator: "gt",
			raw:      json.RawMessage(`5`),
			wantBool: false,
		},
		{
			name:     "lt less true",
			observed: 1,
			operator: "lt",
			raw:      json.RawMessage(`5`),
			wantBool: true,
		},
		{
			name:     "lt equal boundary false",
			observed: 5,
			operator: "lt",
			raw:      json.RawMessage(`5`),
			wantBool: false,
		},
		{
			name:     "lt greater false",
			observed: 10,
			operator: "lt",
			raw:      json.RawMessage(`5`),
			wantBool: false,
		},
		{
			name:     "numeric op observed non numeric parse error",
			observed: "not-a-number",
			operator: "gte",
			raw:      json.RawMessage(`5`),
			wantErr:  true,
		},
		{
			name:     "numeric op expected non numeric parse error",
			observed: 5,
			operator: "gte",
			raw:      json.RawMessage(`"abc"`),
			wantErr:  true,
		},
		// in operator: array membership via fmt.Sprint.
		{
			name:     "in match true",
			observed: "running",
			operator: "in",
			raw:      json.RawMessage(`["running","stopped"]`),
			wantBool: true,
		},
		{
			name:     "in no match false",
			observed: "paused",
			operator: "in",
			raw:      json.RawMessage(`["running","stopped"]`),
			wantBool: false,
		},
		{
			name:     "in empty array false",
			observed: "running",
			operator: "in",
			raw:      json.RawMessage(`[]`),
			wantBool: false,
		},
		{
			name:     "in numeric coercion match true",
			observed: 5,
			operator: "in",
			raw:      json.RawMessage(`[5,"x"]`),
			wantBool: true,
		},
		{
			name:      "in string scalar value error",
			observed:  "running",
			operator:  "in",
			raw:       json.RawMessage(`"running"`),
			wantErr:   true,
			errSubstr: "in predicate requires array value",
		},
		{
			name:      "in object value error",
			observed:  "running",
			operator:  "in",
			raw:       json.RawMessage(`{"a":1}`),
			wantErr:   true,
			errSubstr: "in predicate requires array value",
		},
		// default arm: unsupported operator.
		{
			name:      "unsupported operator error",
			observed:  "running",
			operator:  "weird",
			raw:       json.RawMessage(`"running"`),
			wantErr:   true,
			errSubstr: "unsupported predicate operator",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := comparePredicate(tc.observed, tc.operator, tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("comparePredicate err = nil, want non-nil")
				}
				if tc.errSubstr != "" && !strings.Contains(err.Error(), tc.errSubstr) {
					t.Fatalf("comparePredicate err = %q, want substring %q", err.Error(), tc.errSubstr)
				}
				if got != false {
					t.Fatalf("comparePredicate bool = %v, want false on error path", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("comparePredicate err = %v, want nil", err)
			}
			if got != tc.wantBool {
				t.Fatalf("comparePredicate bool = %v, want %v", got, tc.wantBool)
			}
		})
	}
}
