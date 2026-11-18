package meta

import (
	"database/sql"
	"reflect"
	"time"
	"unsafe"
)

// InUTC returns the function that moves a scanned value of type t to UTC in
// place, or nil for a type that holds no time ant knows. The instant does not
// change, only the zone it is held in: a driver may decode a time in the
// process's zone, as pgx does with TIMESTAMPTZ, and a row read back then
// differs from the one written. It runs after the scan, so NULL handling and
// the scan's errors stay database/sql's.
func InUTC(t reflect.Type) func(unsafe.Pointer) { return utcMovers[t] }

// InUTC moves this field of the entity at entity to UTC, if it holds a time.
func (f *Field) InUTC(entity unsafe.Pointer) {
	if f.utc != nil {
		f.utc(unsafe.Add(entity, f.Offset))
	}
}

// HoldsTime reports whether InUTC changes anything for this field.
func (f *Field) HoldsTime() bool { return f.utc != nil }

// utcMovers covers the time shapes a field can have. UnixTime is not here, as
// its Scan makes a UTC time already.
var utcMovers = map[reflect.Type]func(unsafe.Pointer){
	reflect.TypeFor[time.Time](): func(p unsafe.Pointer) { toUTC((*time.Time)(p)) },
	reflect.TypeFor[*time.Time](): func(p unsafe.Pointer) {
		if t := *(**time.Time)(p); t != nil {
			toUTC(t)
		}
	},
	reflect.TypeFor[sql.NullTime](): func(p unsafe.Pointer) {
		toUTC(&(*sql.NullTime)(p).Time)
	},
	reflect.TypeFor[sql.Null[time.Time]](): func(p unsafe.Pointer) {
		toUTC(&(*sql.Null[time.Time])(p).V)
	},
	reflect.TypeFor[DeletedAt](): func(p unsafe.Pointer) { toUTC(&(*DeletedAt)(p).V) },
	reflect.TypeFor[[]time.Time](): func(p unsafe.Pointer) {
		for i := range *(*[]time.Time)(p) {
			toUTC(&(*(*[]time.Time)(p))[i])
		}
	},
}

func toUTC(t *time.Time) { *t = t.UTC() }
