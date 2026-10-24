package metrics

import (
	"database/sql"
	"fmt"
)

// Retained rows normally carry int64 timestamps/ordinals and float64 values.
// Assign those types without database/sql's per-observation reflection. Keep its
// ordinary numeric conversion for older or atypically typed observations; this
// is an application-side decoder, not a different SQL or driver contract.
type retainedInt64 int64

func (n *retainedInt64) Scan(src any) error {
	if value, ok := src.(int64); ok {
		*n = retainedInt64(value)
		return nil
	}
	var compatible sql.NullInt64
	if err := compatible.Scan(src); err != nil {
		return err
	}
	if !compatible.Valid {
		return fmt.Errorf("converting NULL to int64 is unsupported")
	}
	*n = retainedInt64(compatible.Int64)
	return nil
}

type retainedFloat64 float64

func (n *retainedFloat64) Scan(src any) error {
	if value, ok := src.(float64); ok {
		*n = retainedFloat64(value)
		return nil
	}
	var compatible sql.NullFloat64
	if err := compatible.Scan(src); err != nil {
		return err
	}
	if !compatible.Valid {
		return fmt.Errorf("converting NULL to float64 is unsupported")
	}
	*n = retainedFloat64(compatible.Float64)
	return nil
}
