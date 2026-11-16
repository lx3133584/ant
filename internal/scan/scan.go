// Package scan maps database rows onto Go values.
package scan

import (
	"database/sql"
	"reflect"
	"time"
	"unsafe"

	"github.com/skudra-go/ant/internal/meta"
)

// For returns a function that scans the current row into a *T. A value type
// such as int takes the single result column. A struct takes columns by name:
// columns with no field are ignored, and fields with no column are left alone.
func For[T any](rows *sql.Rows) (func(*T) error, error) {
	if meta.IsValue(reflect.TypeFor[T]()) {
		return func(dst *T) error { return rows.Scan(dst) }, nil
	}

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	model := meta.Of[T]()

	fields := make([]*meta.Field, len(columns))

	for i, c := range columns {
		fields[i] = model.Column(c)
	}

	// The columns holding an instant, found once: a row moves only these, and
	// every other column costs what it did before.
	times := make([]*meta.Field, 0, len(columns))

	for _, f := range fields {
		if f != nil && (f.Type == timeType || f.Type == timeSliceType) {
			times = append(times, f)
		}
	}

	dest := make([]any, len(columns))

	return func(dst *T) error {
		for i, f := range fields {
			if f == nil {
				dest[i] = new(any)

				continue
			}

			dest[i] = f.Addr(unsafe.Pointer(dst))
		}

		// Scanned first, moved to UTC after: what the column holds is
		// database/sql's to read, and a NULL or a value no time comes from is
		// refused there, as it is for any other field.
		if err := rows.Scan(dest...); err != nil {
			return err
		}

		for _, f := range times {
			switch f.Type {
			case timeType:
				one := (*time.Time)(unsafe.Add(unsafe.Pointer(dst), f.Offset))
				*one = one.UTC()
			case timeSliceType:
				slice := *(*[]time.Time)(unsafe.Add(unsafe.Pointer(dst), f.Offset))
				for i := range slice {
					slice[i] = slice[i].UTC()
				}
			}
		}

		return nil
	}, nil
}

// timeType is time.Time, which a dialect may hand back in another zone.
var timeType = reflect.TypeFor[time.Time]()

// timeSliceType is []time.Time, a column of instants on the databases that have
// array columns. Its elements need what a single time needs.
var timeSliceType = reflect.TypeFor[[]time.Time]()
