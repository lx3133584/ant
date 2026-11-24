package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/schema"
)

// Database is a context that embeds ant.Context.
type Database interface {
	DB() *sql.DB
}

// Partitions manages the monthly partitions of a table declared with
// b.PartitionByRange on a time column. Migrations create the parent table, and this
// creates and removes what hangs off it, which is routine work and not a schema
// change. Dropping a month is instant, where deleting its rows would take hours and
// leave the table bloated.
//
//	events := postgres.MonthlyPartitions[Event](db)
//	events.Ensure(ctx, time.Now(), 3)             // this month and the next three
//	events.DropBefore(ctx, cutoff)                // whole months older than cutoff
//
// A month is a partition whose bounds run from midnight UTC on the first of a
// month up to midnight UTC on the first of the next. Its name does not matter:
// a month made under another name, as Pagila's payment_p2022_01, is a month
// here too, so DropBefore drops it. Ensure names the months it makes
// table_2026_10. A partition over another span is not a month and is left
// alone.
type Partitions struct {
	db     *sql.DB
	schema string
	table  string
}

// MonthlyPartitions returns the manager for model T's table.
func MonthlyPartitions[T any](db Database) (*Partitions, error) {
	schemaName, table, err := ant.TableOf[T]()
	if err != nil {
		return nil, err
	}

	return &Partitions{db: db.DB(), schema: schemaName, table: table}, nil
}

func (p *Partitions) name(table string) string {
	return impl{}.table(schema.Ref{Schema: p.schema, Name: table})
}

func monthStart(t time.Time) time.Time {
	t = t.UTC()

	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// Partition is one month of a table.
type Partition struct {
	Name  string
	Month time.Time // midnight UTC on the first
}

// Ensure creates the partitions for the month holding from and the months after
// it, and returns the ones it created. It is safe to run on a schedule, and from
// several instances at once.
//
// A month another partition already covers is not created again, whatever that
// partition is named. A partition whose name is the one Ensure would give a
// month, but whose bounds are not that month, is an error rather than a month
// left uncovered: CREATE TABLE IF NOT EXISTS would skip it and report success.
func (p *Partitions) Ensure(
	ctx context.Context,
	from time.Time,
	monthsAhead int,
) ([]string, error) {
	existing, err := p.children(ctx)
	if err != nil {
		return nil, err
	}

	var created []string

	start := monthStart(from)

	for i := 0; i <= monthsAhead; i++ {
		month := start.AddDate(0, i, 0)
		name := fmt.Sprintf("%s_%04d_%02d", p.table, month.Year(), month.Month())

		taken := false

		for _, c := range existing {
			if c.month != nil && c.month.Equal(month) {
				taken = true
			}

			if c.name == name && (c.month == nil || !c.month.Equal(month)) {
				return created, fmt.Errorf(
					"postgres: %s is taken by a partition that is not the month of %s",
					name,
					month.Format("2006-01"),
				)
			}
		}

		if taken {
			continue
		}

		statement := fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')",
			p.name(name),
			p.name(p.table),
			month.Format("2006-01-02 15:04:05Z07:00"),
			month.AddDate(0, 1, 0).Format("2006-01-02 15:04:05Z07:00"),
		)

		if _, err := p.db.ExecContext(ctx, statement); err != nil {
			return created, fmt.Errorf("postgres: create partition %s: %w", name, err)
		}

		created = append(created, name)
	}

	return created, nil
}

// List returns the monthly partitions, oldest first.
//
// A partition is a month of the table when its bounds are one: from midnight
// UTC on the first of a month up to midnight UTC on the first of the next. Its
// name does not matter. A month made before the table was ant's, as Pagila's
// payment_p2022_01, is a month like any other, so a schedule that lists and
// drops months covers it (#196). A partition over another span is left alone.
func (p *Partitions) List(ctx context.Context) ([]Partition, error) {
	all, err := p.children(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]Partition, 0, len(all))

	for _, c := range all {
		if c.month != nil {
			out = append(out, Partition{Name: c.name, Month: *c.month})
		}
	}

	slices.SortFunc(out, func(a, b Partition) int { return a.Month.Compare(b.Month) })

	return out, nil
}

// child is a partition the parent has, with the month it is when its bounds
// are exactly one.
type child struct {
	name  string
	month *time.Time // set when the bounds are a whole month
}

// children returns every partition of the parent, with the month each one is
// when its bounds are exactly one, oldest first among those. It reads the
// bounds inside one transaction with DateStyle pinned, so the text has one
// shape to parse whatever the server or the caller set.
func (p *Partitions) children(ctx context.Context) ([]child, error) {
	tx, err := p.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("postgres: list partitions: %w", err)
	}

	defer tx.Rollback()

	// A partition's bounds are text, and the way PostgreSQL writes a timestamp
	// in that text follows DateStyle.
	if _, err := tx.ExecContext(ctx, "SET LOCAL datestyle TO 'ISO, MDY'"); err != nil {
		return nil, fmt.Errorf("postgres: list partitions: %w", err)
	}

	rows, err := tx.QueryContext(
		ctx,
		`
		SELECT c.relname, COALESCE(pg_get_expr(c.relpartbound, c.oid), '')
		FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = to_regclass($1)`,
		p.name(p.table),
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: list partitions: %w", err)
	}

	var out []child

	err = each(rows, func() error {
		var name, bound string
		if err := rows.Scan(&name, &bound); err != nil {
			return err
		}

		out = append(out, child{name: name, month: monthOf(bound)})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: list partitions: %w", err)
	}

	return out, nil
}

// monthOf returns the month a partition's bounds are, when they are exactly
// one: from midnight UTC on the first of a month up to midnight UTC on the
// first of the next. Any other span, or one with MINVALUE or MAXVALUE at an
// end, is not a month.
func monthOf(bound string) *time.Time {
	m := rangeBounds.FindStringSubmatch(bound)
	if m == nil {
		return nil
	}

	from, ok := parseBound(m[1])
	if !ok {
		return nil
	}

	to, ok := parseBound(m[2])
	if !ok {
		return nil
	}

	month := monthStart(from)
	if !from.Equal(month) || !to.Equal(month.AddDate(0, 1, 0)) {
		return nil
	}

	return &month
}

// rangeBounds matches the bounds PostgreSQL prints for a range partition, as
// pg_get_expr gives them: FOR VALUES FROM ('...') TO ('...'). A bound the
// keywords MINVALUE or MAXVALUE stand in for has no quotes, so it does not
// match, and such a partition is not a month.
var rangeBounds = regexp.MustCompile(`^FOR VALUES FROM \('([^']+)'\) TO \('([^']+)'\)$`)

// parseBound reads a timestamp from a partition's bounds. DateStyle ISO writes
// it with a zone for a timestamptz column, as 2026-04-01 00:00:00+00, without
// one for a timestamp column, as 2026-04-01 00:00:00, and as a plain date for
// a date column, as 2026-04-01. A fraction may follow the seconds of the
// timestamp forms. A bound with no zone comes from a column that holds none,
// and ant partitions on UTC, so it is read as UTC.
func parseBound(s string) (time.Time, bool) {
	zoned := []string{
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05.999999999-07:00",
	}

	utc := []string{
		"2006-01-02 15:04:05.999999999",
		"2006-01-02",
	}

	for _, layout := range zoned {
		if at, err := time.Parse(layout, s); err == nil {
			return at.UTC(), true
		}
	}

	for _, layout := range utc {
		if at, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return at, true
		}
	}

	return time.Time{}, false
}

// DropBefore removes every partition whose whole month ended at or before cutoff,
// with its rows, and returns their names. A month partly after cutoff stays, so no
// row newer than cutoff is lost. When another partitioned table has a foreign key to
// this one, call DropBefore on that table first.
func (p *Partitions) DropBefore(ctx context.Context, cutoff time.Time) ([]string, error) {
	existing, err := p.List(ctx)
	if err != nil {
		return nil, err
	}

	var dropped []string

	for _, partition := range existing {
		if partition.Month.AddDate(0, 1, 0).After(cutoff) {
			continue
		}
		// Detached first, because a table that other partitions refer to cannot
		// be dropped directly, and a detach checks no rows still need it.
		tx, err := p.db.BeginTx(ctx, nil)
		if err != nil {
			return dropped, err
		}

		_, err = tx.ExecContext(
			ctx,
			fmt.Sprintf(
				"ALTER TABLE %s DETACH PARTITION %s",
				p.name(p.table),
				p.name(partition.Name),
			),
		)
		if err == nil {
			_, err = tx.ExecContext(ctx, "DROP TABLE "+p.name(partition.Name))
		}

		if err != nil {
			tx.Rollback()

			return dropped, fmt.Errorf("postgres: drop partition %s: %w", partition.Name, err)
		}

		if err := tx.Commit(); err != nil {
			return dropped, err
		}

		dropped = append(dropped, partition.Name)
	}

	return dropped, nil
}
