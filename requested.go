package provider

import (
	"reflect"
	"slices"
)

// The types a container request is about. Each mirrors what the container
// accepts and returns nothing for a shape it would reject, leaving the container
// to report the error.

// resolved returns the type Resolve is asked for.
func resolved(abstraction any) []reflect.Type {
	t := reflect.TypeOf(abstraction)
	if t == nil || t.Kind() != reflect.Pointer {
		return nil
	}

	return []reflect.Type{t.Elem()}
}

// arguments returns the types Call needs to invoke the receiver.
func arguments(receiver any) []reflect.Type {
	t := reflect.TypeOf(receiver)
	if t == nil || t.Kind() != reflect.Func {
		return nil
	}

	return slices.Collect(t.Ins())
}

// injected returns the types Fill injects into the receiver's tagged fields.
func injected(receiver any) []reflect.Type {
	t := reflect.TypeOf(receiver)
	if t == nil || t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
		return nil
	}

	elem := t.Elem()

	var types []reflect.Type
	for i := range elem.NumField() {
		field := elem.Field(i)
		if _, tagged := field.Tag.Lookup("container"); tagged {
			types = append(types, field.Type)
		}
	}

	return types
}
