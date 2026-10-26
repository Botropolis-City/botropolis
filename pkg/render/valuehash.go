package render

import (
	"encoding/binary"
	"hash/maphash"
	"math"
	"reflect"
	"sync"
	"time"
	"unsafe"
)

// valueHash hashes v by value, every field of it, exported or not, and
// whatever its pointers point at, so a field added to what a drawable is
// drawn from joins its key without anyone listing it (item 212). A map
// hashes the same whatever order it is walked in; a time is its instant
// and its location's identity.
//
// It hashes through a plan made once for v's type, which reads v's
// memory as walking it by reflection would and writes the same; a map,
// an interface and what neither covers are walked by reflection still.
func valueHash(v any) uint64 {
	var h maphash.Hash
	h.SetSeed(inkSeed)
	w := valueWriter{h: &h}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		w.put(0)
		return h.Sum64()
	}
	at := reflect.New(rv.Type())
	at.Elem().Set(rv)
	planOf(rv.Type())(&w, at.UnsafePointer())
	return h.Sum64()
}

// hashOf is valueHash of *v, hashed where it is rather than copied.
func hashOf[T any](v *T) uint64 {
	var h maphash.Hash
	h.SetSeed(inkSeed)
	w := valueWriter{h: &h}
	planOf(reflect.TypeFor[T]())(&w, unsafe.Pointer(v))
	return h.Sum64()
}

// reflectHash is valueHash walking v by reflection, as its plans must.
func reflectHash(v any) uint64 {
	var h maphash.Hash
	h.SetSeed(inkSeed)
	w := valueWriter{h: &h}
	w.value(reflect.ValueOf(v))
	return h.Sum64()
}

// hashPlan writes the value of its type at p.
type hashPlan func(w *valueWriter, p unsafe.Pointer)

// planCell holds a type's plan, so a type that refers to itself can be
// planned before its plan is done.
type planCell struct{ plan hashPlan }

var plans sync.Map // reflect.Type → *planCell

// planOf is t's plan, made the first time it is asked for.
func planOf(t reflect.Type) hashPlan {
	if c, ok := plans.Load(t); ok {
		cell := c.(*planCell)
		if cell.plan != nil {
			return cell.plan
		}
		return func(w *valueWriter, p unsafe.Pointer) { cell.plan(w, p) }
	}
	cell := &planCell{}
	if c, loaded := plans.LoadOrStore(t, cell); loaded {
		cell = c.(*planCell)
		return func(w *valueWriter, p unsafe.Pointer) { cell.plan(w, p) }
	}
	cell.plan = makePlan(t)
	return cell.plan
}

func makePlan(t reflect.Type) hashPlan {
	if t == timeType {
		wall, ext, loc := t.Field(0).Offset, t.Field(1).Offset, t.Field(2).Offset
		return func(w *valueWriter, p unsafe.Pointer) {
			w.put(*(*uint64)(unsafe.Add(p, wall)))
			w.put(uint64(*(*int64)(unsafe.Add(p, ext))))
			w.put(uint64(*(*uintptr)(unsafe.Add(p, loc))))
		}
	}
	switch t.Kind() {
	case reflect.Bool:
		return func(w *valueWriter, p unsafe.Pointer) {
			if *(*bool)(p) {
				w.put(1)
			} else {
				w.put(0)
			}
		}
	case reflect.Int:
		return func(w *valueWriter, p unsafe.Pointer) { w.put(uint64(int64(*(*int)(p)))) }
	case reflect.Int8:
		return func(w *valueWriter, p unsafe.Pointer) { w.put(uint64(int64(*(*int8)(p)))) }
	case reflect.Int16:
		return func(w *valueWriter, p unsafe.Pointer) { w.put(uint64(int64(*(*int16)(p)))) }
	case reflect.Int32:
		return func(w *valueWriter, p unsafe.Pointer) { w.put(uint64(int64(*(*int32)(p)))) }
	case reflect.Int64:
		return func(w *valueWriter, p unsafe.Pointer) { w.put(uint64(*(*int64)(p))) }
	case reflect.Uint:
		return func(w *valueWriter, p unsafe.Pointer) { w.put(uint64(*(*uint)(p))) }
	case reflect.Uint8:
		return func(w *valueWriter, p unsafe.Pointer) { w.put(uint64(*(*uint8)(p))) }
	case reflect.Uint16:
		return func(w *valueWriter, p unsafe.Pointer) { w.put(uint64(*(*uint16)(p))) }
	case reflect.Uint32:
		return func(w *valueWriter, p unsafe.Pointer) { w.put(uint64(*(*uint32)(p))) }
	case reflect.Uint64:
		return func(w *valueWriter, p unsafe.Pointer) { w.put(*(*uint64)(p)) }
	case reflect.Uintptr:
		return func(w *valueWriter, p unsafe.Pointer) { w.put(uint64(*(*uintptr)(p))) }
	case reflect.Float32:
		return func(w *valueWriter, p unsafe.Pointer) { w.put(math.Float64bits(float64(*(*float32)(p)))) }
	case reflect.Float64:
		return func(w *valueWriter, p unsafe.Pointer) { w.put(math.Float64bits(*(*float64)(p))) }
	case reflect.String:
		return func(w *valueWriter, p unsafe.Pointer) {
			s := *(*string)(p)
			w.put(uint64(len(s)))
			w.h.WriteString(s)
		}
	case reflect.Struct:
		type field struct {
			off  uintptr
			plan hashPlan
		}
		fields := make([]field, t.NumField())
		for i := range fields {
			f := t.Field(i)
			fields[i] = field{f.Offset, planOf(f.Type)}
		}
		return func(w *valueWriter, p unsafe.Pointer) {
			for _, f := range fields {
				f.plan(w, unsafe.Add(p, f.off))
			}
		}
	case reflect.Array:
		elem, size, n := planOf(t.Elem()), t.Elem().Size(), t.Len()
		return func(w *valueWriter, p unsafe.Pointer) {
			for i := range n {
				elem(w, unsafe.Add(p, uintptr(i)*size))
			}
		}
	case reflect.Slice:
		elem, size := planOf(t.Elem()), t.Elem().Size()
		return func(w *valueWriter, p unsafe.Pointer) {
			data, n := *(*unsafe.Pointer)(p), *(*int)(unsafe.Add(p, unsafe.Sizeof(uintptr(0))))
			if data == nil {
				w.put(math.MaxUint64)
				return
			}
			w.put(uint64(n))
			for i := range n {
				elem(w, unsafe.Add(data, uintptr(i)*size))
			}
		}
	case reflect.Pointer:
		elem := t.Elem()
		return func(w *valueWriter, p unsafe.Pointer) {
			to := *(*unsafe.Pointer)(p)
			if to == nil {
				w.put(0)
				return
			}
			at := uintptr(to)
			if w.seen[at] {
				w.put(2)
				return
			}
			if w.seen == nil {
				w.seen = map[uintptr]bool{}
			}
			w.seen[at] = true
			w.put(1)
			planOf(elem)(w, to)
			delete(w.seen, at)
		}
	}
	// A map, an interface, a func, a channel: walked by reflection.
	return func(w *valueWriter, p unsafe.Pointer) { w.value(reflect.NewAt(t, p).Elem()) }
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
