// Package scan maps database rows onto Go values.
package scan

import (
	"database/sql"
	"fmt"
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

	dest := make([]any, len(columns))

	return func(dst *T) error {
		for i, f := range fields {
			switch {
			case f == nil:
				dest[i] = new(any)
			case f.Type == timeType:
				dest[i] = (*utcTime)(f.Addr(unsafe.Pointer(dst)).(*time.Time))
			default:
				dest[i] = f.Addr(unsafe.Pointer(dst))
			}
		}

		return rows.Scan(dest...)
	}, nil
}

// timeType is time.Time, which a dialect may hand back in another zone.
var timeType = reflect.TypeFor[time.Time]()

// utcTime scans a time and stores it in UTC. The drivers disagree: pgx decodes
// TIMESTAMPTZ into the process's local zone, while MySQL and SQLite return UTC,
// so without this the same row reads back as two different Go values.
type utcTime time.Time

func (t *utcTime) Scan(src any) error {
	var v time.Time

	switch s := src.(type) {
	case time.Time:
		v = s
	case nil:
		*t = utcTime(time.Time{})

		return nil
	default:
		return fmt.Errorf("ant: cannot scan %T into time.Time", src)
	}

	*t = utcTime(v.UTC())

	return nil
}
