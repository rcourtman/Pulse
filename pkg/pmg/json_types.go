package pmg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// flexibleFloat handles numeric values that may arrive as numbers, strings, or nulls.
type flexibleFloat float64

func (f *flexibleFloat) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*f = 0
		return nil
	}

	// Try decoding as a JSON number first
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		*f = flexibleFloat(num)
		return nil
	}

	// Fall back to string decoding and parsing
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return fmt.Errorf("invalid numeric value %q: %w", string(data), err)
	}

	str = strings.TrimSpace(str)
	if str == "" || strings.EqualFold(str, "null") {
		*f = 0
		return nil
	}

	parsed, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return fmt.Errorf("failed to parse numeric string %q: %w", str, err)
	}

	*f = flexibleFloat(parsed)
	return nil
}

func (f flexibleFloat) Float64() float64 {
	return float64(f)
}

// flexibleInt handles integer values that may arrive as ints, floats, strings, or nulls.
type flexibleInt int64

func (i *flexibleInt) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*i = 0
		return nil
	}

	// Try integer decoding
	var num int64
	if err := json.Unmarshal(data, &num); err == nil {
		*i = flexibleInt(num)
		return nil
	}

	// Try float decoding (round towards zero)
	var floatNum float64
	if err := json.Unmarshal(data, &floatNum); err == nil {
		*i = flexibleInt(int64(floatNum))
		return nil
	}

	// Fall back to string parsing
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return fmt.Errorf("invalid integer value %q: %w", string(data), err)
	}

	str = strings.TrimSpace(str)
	if str == "" || strings.EqualFold(str, "null") {
		*i = 0
		return nil
	}

	parsed, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return fmt.Errorf("failed to parse integer string %q: %w", str, err)
	}

	*i = flexibleInt(int64(parsed))
	return nil
}

func (i flexibleInt) Int64() int64 {
	return int64(i)
}

func (i flexibleInt) Int() int {
	return int(i)
}
