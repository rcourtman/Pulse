package config

import (
	"reflect"
	"testing"
	"time"
)

var timeType = reflect.TypeOf(time.Time{})

// Every field is filled, so a map, slice or pointer added to the config later
// fails here until Clone copies it.
func TestAlertConfigCloneSharesNothingWithTheOriginal(t *testing.T) {
	var original AlertConfig
	fillForClone(reflect.ValueOf(&original).Elem())

	clone := original.Clone()

	if !reflect.DeepEqual(clone, original) {
		t.Fatalf("clone differs from the original:\n got %+v\nwant %+v", clone, original)
	}
	assertNothingShared(t, "AlertConfig", reflect.ValueOf(original), reflect.ValueOf(clone))
}

func TestAlertConfigCloneKeepsNilAndEmptyApart(t *testing.T) {
	original := AlertConfig{
		Overrides:   map[string]ThresholdConfig{},
		CustomRules: []CustomAlertRule{{FilterConditions: FilterStack{Filters: []FilterCondition{{Value: []interface{}(nil)}}}}},
	}

	clone := original.Clone()

	if !reflect.DeepEqual(clone, original) {
		t.Fatalf("clone differs from the original:\n got %#v\nwant %#v", clone, original)
	}
	if clone.Overrides == nil {
		t.Fatal("clone turned an empty overrides map into nil")
	}
	if clone.TimeThresholds != nil {
		t.Fatal("clone turned a nil delay map into an empty one")
	}
}

func fillForClone(value reflect.Value) {
	switch value.Kind() {
	case reflect.Bool:
		value.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value.SetInt(7)
	case reflect.Float32, reflect.Float64:
		value.SetFloat(7.5)
	case reflect.String:
		value.SetString("filled")
	case reflect.Pointer:
		value.Set(reflect.New(value.Type().Elem()))
		fillForClone(value.Elem())
	case reflect.Map:
		value.Set(reflect.MakeMap(value.Type()))
		key := reflect.New(value.Type().Key()).Elem()
		fillForClone(key)
		item := reflect.New(value.Type().Elem()).Elem()
		fillForClone(item)
		value.SetMapIndex(key, item)
	case reflect.Slice:
		value.Set(reflect.MakeSlice(value.Type(), 1, 1))
		fillForClone(value.Index(0))
	case reflect.Interface:
		// Filter values arrive from encoding/json.
		value.Set(reflect.ValueOf(map[string]interface{}{"filled": []interface{}{"filled", 7.5}}))
	case reflect.Struct:
		if value.Type() == timeType {
			value.Set(reflect.ValueOf(time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)))
			return
		}
		for i := 0; i < value.NumField(); i++ {
			if value.Field(i).CanSet() {
				fillForClone(value.Field(i))
			}
		}
	}
}

func assertNothingShared(t *testing.T, path string, original, clone reflect.Value) {
	t.Helper()
	switch original.Kind() {
	case reflect.Pointer:
		if original.IsNil() {
			return
		}
		if original.Pointer() == clone.Pointer() {
			t.Errorf("%s: clone shares the pointer", path)
			return
		}
		assertNothingShared(t, path, original.Elem(), clone.Elem())
	case reflect.Map:
		if original.IsNil() {
			return
		}
		if original.Pointer() == clone.Pointer() {
			t.Errorf("%s: clone shares the map", path)
			return
		}
		iter := original.MapRange()
		for iter.Next() {
			assertNothingShared(t, path+"[key]", iter.Value(), clone.MapIndex(iter.Key()))
		}
	case reflect.Slice:
		if original.Len() == 0 {
			return
		}
		if original.Pointer() == clone.Pointer() {
			t.Errorf("%s: clone shares the slice", path)
			return
		}
		for i := 0; i < original.Len(); i++ {
			assertNothingShared(t, path+"[i]", original.Index(i), clone.Index(i))
		}
	case reflect.Interface:
		if original.IsNil() {
			return
		}
		assertNothingShared(t, path, original.Elem(), clone.Elem())
	case reflect.Struct:
		if original.Type() == timeType {
			return
		}
		for i := 0; i < original.NumField(); i++ {
			if original.Type().Field(i).IsExported() {
				assertNothingShared(t, path+"."+original.Type().Field(i).Name, original.Field(i), clone.Field(i))
			}
		}
	}
}
