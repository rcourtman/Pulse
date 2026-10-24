package metrics

import (
	"math"
	"testing"
)

// Read each value through database/sql twice, once with its primitive decoder
// and once through the retained decoder. The fallback must accept and reject
// exactly the same data, not turn malformed/NULL observations into zeroes.
func TestRetainedNumericScansMatchDatabaseSQL(t *testing.T) {
	db := newPlanTestDB(t)
	cases := []struct {
		name  string
		value any
	}{
		{"zero", int64(0)}, {"negative", int64(-201)},
		{"max-int64", int64(math.MaxInt64)}, {"min-int64", int64(math.MinInt64)},
		{"outside-exact-float-integer", int64(1<<53 + 1)},
		{"float", 17.125}, {"negative-float", -37.5},
		{"negative-zero", math.Copysign(0, -1)},
		{"tiny-float", math.SmallestNonzeroFloat64}, {"large-float", math.MaxFloat64},
		{"integer-text", "-123"}, {"float-text", "19.25"},
		{"integer-blob", []byte("234")}, {"float-blob", []byte("1.25e3")},
		{"overflow-int", "9223372036854775808"}, {"invalid", "not a number"},
		{"fractional-int", "1.5"}, {"empty", ""}, {"null", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var wantInt int64
			var gotInt retainedInt64 = -777
			wantErr := db.QueryRow("SELECT ?", tc.value).Scan(&wantInt)
			gotErr := db.QueryRow("SELECT ?", tc.value).Scan(&gotInt)
			if (wantErr == nil) != (gotErr == nil) || (wantErr == nil && int64(gotInt) != wantInt) {
				t.Fatalf("integer: standard=%d/%v retained=%d/%v", wantInt, wantErr, gotInt, gotErr)
			}
			if wantErr != nil && gotInt != -777 {
				t.Fatalf("invalid integer changed destination to %d", gotInt)
			}
			var wantFloat float64
			var gotFloat retainedFloat64 = -777
			wantErr = db.QueryRow("SELECT ?", tc.value).Scan(&wantFloat)
			gotErr = db.QueryRow("SELECT ?", tc.value).Scan(&gotFloat)
			if (wantErr == nil) != (gotErr == nil) || (wantErr == nil && math.Float64bits(float64(gotFloat)) != math.Float64bits(wantFloat)) {
				t.Fatalf("float: standard=%g/%v retained=%g/%v", wantFloat, wantErr, gotFloat, gotErr)
			}
			if wantErr != nil && gotFloat != -777 {
				t.Fatalf("invalid float changed destination to %g", gotFloat)
			}
		})
	}
}

func TestRetainedNativeNumericScansDoNotAllocate(t *testing.T) {
	var number retainedInt64
	var value retainedFloat64
	var ordinal any = int64(149)
	var timestamp any = int64(1792854756)
	var observation any = 12.5
	allocations := testing.AllocsPerRun(100, func() {
		if err := number.Scan(ordinal); err != nil {
			panic(err)
		}
		if err := number.Scan(timestamp); err != nil {
			panic(err)
		}
		if err := value.Scan(observation); err != nil {
			panic(err)
		}
	})
	if allocations != 0 || number != 1792854756 || value != 12.5 {
		t.Fatalf("native values: allocations=%g number=%d value=%g", allocations, number, value)
	}
}
