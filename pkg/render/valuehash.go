package render

import (
	"encoding/binary"
	"hash/maphash"
	"math"
	"reflect"
	"time"
)

// valueHash hashes v by value, every field of it, exported or not, and
// whatever its pointers point at, so a field added to what a drawable is
// drawn from joins its key without anyone listing it (item 212). A map
// hashes the same whatever order it is walked in; a time is its instant
// and its location's identity.
func valueHash(v any) uint64 {
	var h maphash.Hash
	h.SetSeed(inkSeed)
	w := valueWriter{h: &h}
	w.value(reflect.ValueOf(v))
	return h.Sum64()
}

type valueWriter struct {
	h    *maphash.Hash
	seen map[uintptr]bool
	b    [8]byte
}

var timeType = reflect.TypeFor[time.Time]()

func (w *valueWriter) put(v uint64) {
	binary.LittleEndian.PutUint64(w.b[:], v)
	w.h.Write(w.b[:])
}

func (w *valueWriter) value(v reflect.Value) {
	if !v.IsValid() {
		w.put(0)
		return
	}
	if v.Type() == timeType {
		w.put(v.Field(0).Uint())
		w.put(uint64(v.Field(1).Int()))
		w.put(uint64(v.Field(2).Pointer()))
		return
	}
	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			w.put(1)
		} else {
			w.put(0)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		w.put(uint64(v.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		w.put(v.Uint())
	case reflect.Float32, reflect.Float64:
		w.put(math.Float64bits(v.Float()))
	case reflect.Complex64, reflect.Complex128:
		c := v.Complex()
		w.put(math.Float64bits(real(c)))
		w.put(math.Float64bits(imag(c)))
	case reflect.String:
		w.put(uint64(v.Len()))
		w.h.WriteString(v.String())
	case reflect.Struct:
		for i := range v.NumField() {
			w.value(v.Field(i))
		}
	case reflect.Array:
		for i := range v.Len() {
			w.value(v.Index(i))
		}
	case reflect.Slice:
		if v.IsNil() {
			w.put(math.MaxUint64)
			return
		}
		w.put(uint64(v.Len()))
		for i := range v.Len() {
			w.value(v.Index(i))
		}
	case reflect.Map:
		if v.IsNil() {
			w.put(math.MaxUint64)
			return
		}
		w.put(uint64(v.Len()))
		var sum uint64
		it := v.MapRange()
		for it.Next() {
			var h maphash.Hash
			h.SetSeed(inkSeed)
			pair := valueWriter{h: &h, seen: w.seen}
			pair.value(it.Key())
			pair.value(it.Value())
			sum += h.Sum64()
		}
		w.put(sum)
	case reflect.Pointer:
		if v.IsNil() {
			w.put(0)
			return
		}
		p := v.Pointer()
		if w.seen[p] {
			w.put(2)
			return
		}
		if w.seen == nil {
			w.seen = map[uintptr]bool{}
		}
		w.seen[p] = true
		w.put(1)
		w.value(v.Elem())
		delete(w.seen, p)
	case reflect.Interface:
		if v.IsNil() {
			w.put(0)
			return
		}
		w.h.WriteString(v.Elem().Type().String())
		w.value(v.Elem())
	default:
		// A func, a channel or an unsafe pointer: which one it is.
		w.put(uint64(v.Pointer()))
	}
}
