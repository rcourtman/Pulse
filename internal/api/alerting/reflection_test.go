package alerting

import (
	"reflect"
	"testing"
	"unsafe"
)

func setUnexportedField(t *testing.T, target any, fieldName string, value any) {
	t.Helper()
	field := reflect.ValueOf(target).Elem().FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("field %q not found", fieldName)
	}
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}
