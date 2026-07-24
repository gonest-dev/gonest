package accessor

import (
	"reflect"
	"strings"
)

type dirtyValue struct {
	val reflect.Value
}

// SyncAccessorFields inspects src and dst (which must be structs or pointers to structs).
// For every exported field in src that is an Accessor[T] and is dirty (IsDirty() == true),
// it locates the matching field in dst (by struct field name or json tag name) and sets its value.
// If the field in dst is also an Accessor[T], it calls Set(val) on it.
// If the field in dst is a raw type T or *T, it assigns the value directly.
func SyncAccessorFields(dst any, src any) {
	if dst == nil || src == nil {
		return
	}

	srcVal := reflect.ValueOf(src)
	dstVal := reflect.ValueOf(dst)

	if srcVal.Kind() == reflect.Pointer {
		if srcVal.IsNil() {
			return
		}
		srcVal = srcVal.Elem()
	}

	if dstVal.Kind() == reflect.Pointer {
		if dstVal.IsNil() {
			return
		}
		dstVal = dstVal.Elem()
	}

	if srcVal.Kind() != reflect.Struct || dstVal.Kind() != reflect.Struct {
		return
	}

	srcFields := collectDirtyAccessors(srcVal)
	if len(srcFields) == 0 {
		return
	}

	applyToDst(dstVal, srcFields)
}

func collectDirtyAccessors(v reflect.Value) map[string]dirtyValue {
	result := make(map[string]dirtyValue)
	collectDirtyAccessorsRecursive(v, result)
	return result
}

func collectDirtyAccessorsRecursive(v reflect.Value, result map[string]dirtyValue) {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() && !sf.Anonymous {
			continue
		}

		fv := v.Field(i)

		if sf.Anonymous {
			collectDirtyAccessorsRecursive(fv, result)
			continue
		}

		if isDirty(fv) {
			val := getAccessorValue(fv)
			if val.IsValid() {
				dv := dirtyValue{val: val}
				result[sf.Name] = dv

				jsonTag := parseJSONTag(sf.Tag.Get("json"))
				if jsonTag != "" && jsonTag != "-" {
					result[jsonTag] = dv
				}
			}
		}
	}
}

func isDirty(v reflect.Value) bool {
	if !v.IsValid() {
		return false
	}
	m := v.MethodByName("IsDirty")
	if !m.IsValid() && v.CanAddr() {
		m = v.Addr().MethodByName("IsDirty")
	}
	if m.IsValid() {
		res := m.Call(nil)
		if len(res) == 1 && res[0].Kind() == reflect.Bool {
			return res[0].Bool()
		}
	}
	return false
}

func getAccessorValue(v reflect.Value) reflect.Value {
	m := v.MethodByName("Get")
	if !m.IsValid() && v.CanAddr() {
		m = v.Addr().MethodByName("Get")
	}
	if m.IsValid() {
		res := m.Call(nil)
		if len(res) == 1 {
			return res[0]
		}
	}
	return reflect.Value{}
}

func applyToDst(v reflect.Value, srcFields map[string]dirtyValue) {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() && !sf.Anonymous {
			continue
		}

		fv := v.Field(i)

		if sf.Anonymous {
			applyToDst(fv, srcFields)
			continue
		}

		jsonTag := parseJSONTag(sf.Tag.Get("json"))
		var dv dirtyValue
		var ok bool

		if dv, ok = srcFields[sf.Name]; !ok && jsonTag != "" && jsonTag != "-" {
			dv, ok = srcFields[jsonTag]
		}

		if !ok || !dv.val.IsValid() {
			continue
		}

		setTargetValue(fv, dv.val)
	}
}

func setTargetValue(target reflect.Value, val reflect.Value) {
	if !target.CanSet() && target.CanAddr() {
		target = target.Addr()
	}

	m := target.MethodByName("Set")
	if !m.IsValid() && target.CanAddr() {
		m = target.Addr().MethodByName("Set")
	}
	if m.IsValid() {
		paramType := m.Type().In(0)
		if val.Type().AssignableTo(paramType) || val.Type().ConvertibleTo(paramType) {
			m.Call([]reflect.Value{val.Convert(paramType)})
			return
		}
	}

	if target.Kind() != reflect.Pointer && target.CanSet() {
		if val.Type().AssignableTo(target.Type()) {
			target.Set(val)
			return
		} else if val.Type().ConvertibleTo(target.Type()) {
			target.Set(val.Convert(target.Type()))
			return
		}
	}

	if target.Kind() == reflect.Pointer && target.CanSet() {
		elemType := target.Type().Elem()
		if val.Type().AssignableTo(elemType) || val.Type().ConvertibleTo(elemType) {
			if target.IsNil() {
				target.Set(reflect.New(elemType))
			}
			target.Elem().Set(val.Convert(elemType))
			return
		}
	}
}

func parseJSONTag(tag string) string {
	if tag == "" {
		return ""
	}
	parts := strings.Split(tag, ",")
	return strings.TrimSpace(parts[0])
}
