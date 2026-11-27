package migrations

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"
	"github.com/skudra-go/ant/migrate"
)

// Ticket uses a native enum, a sequence, and comments.
type Ticket struct {
	ID     int64
	Number int64
	Status string
	Code   string
}

func (Ticket) Configure(b *ant.ModelBuilder[Ticket]) {
	b.Comment("Support tickets")
	b.Column(&Tickets.Number).Sequence("ticket_numbers", ant.StartAt(1000))

	b.Column(&Tickets.Status).
		Enum("ticket_status", "open", "closed").
		Default("'open'").
		Comment("Where it stands")

	b.Column(&Tickets.Code).Fixed(4)
}

// Tickets is Ticket's probe.
var Tickets = ant.Fields[Ticket]()

// TicketDB holds it.
type TicketDB struct {
	ant.Context
	Tickets ant.Table[Ticket]
}

// Ticket2 adds an enum value.
type Ticket2 struct {
	ID     int64
	Number int64
	Status string
	Code   string
}

func (Ticket2) Configure(b *ant.ModelBuilder[Ticket2]) {
	b.Table("tickets")
	b.Comment("Support tickets")
	b.Column(&Ticket2s.Number).Sequence("ticket_numbers", ant.StartAt(1000))

	b.Column(&Ticket2s.Status).
		Enum("ticket_status", "open", "pending", "closed").
		Default("'open'").
		Comment("Where it stands")

	b.Column(&Ticket2s.Code).Fixed(4)
}

// Ticket2s is Ticket2's probe.
var Ticket2s = ant.Fields[Ticket2]()

// Ticket2DB holds it.
type Ticket2DB struct {
	ant.Context
	Tickets ant.Table[Ticket2]
}

// Ticket3 drops an enum value.
type Ticket3 struct {
	ID     int64
	Number int64
	Status string
	Code   string
}

func (Ticket3) Configure(b *ant.ModelBuilder[Ticket3]) {
	b.Table("tickets")
	b.Comment("Support tickets")
	b.Column(&Ticket3s.Number).Sequence("ticket_numbers", ant.StartAt(1000))

	b.Column(&Ticket3s.Status).
		Enum("ticket_status", "open", "pending").
		Default("'open'").
		Comment("Where it stands")

	b.Column(&Ticket3s.Code).Fixed(4)
}

// Ticket3s is Ticket3's probe.
var Ticket3s = ant.Fields[Ticket3]()

// Ticket3DB holds it.
type Ticket3DB struct {
	ant.Context
	Tickets ant.Table[Ticket3]
}

// Migrations create enums and sequences before the tables that use them.
func TestEnumsSequencesAndComments(t *testing.T) {
	each(t, func(t *testing.T, p *project) {
		must[TicketDB](p, "diff", "start")
		text := p.read("versions/0001_start.up.sql")
		require.Contains(t, text, `CREATE TYPE "ticket_status" AS ENUM ('open', 'closed')`)
		require.Contains(t, text, `CREATE SEQUENCE "ticket_numbers" START WITH 1000`)
		require.Contains(t, text, `"code" char(4) NOT NULL`)
		require.Contains(t, text, `COMMENT ON TABLE "tickets" IS 'Support tickets'`)
		must[TicketDB](p, "up")

		t.Run("the database fills what the model leaves zero", func(t *testing.T) {
			db := open[TicketDB](p)
			ticket := &Ticket{Code: "abcd"}
			require.NoError(t, db.Tickets.Insert(t.Context(), ticket))
			require.EqualValues(t, 1000, ticket.Number)
			require.Equal(t, "open", ticket.Status)
		})

		t.Run("drift sees nothing wrong", func(t *testing.T) {
			require.Contains(t, must[TicketDB](p, "drift"), "No drift")
		})

		t.Run("drift names both sides of a sequence changed by hand", func(t *testing.T) {
			p.sql("ALTER SEQUENCE ticket_numbers INCREMENT BY 5")
			out, err := run[TicketDB](p, "drift")
			require.ErrorIs(t, err, migrate.ErrDrift)
			require.Contains(t, out, "change sequence ticket_numbers: increment 5 to 1")
			p.sql("ALTER SEQUENCE ticket_numbers INCREMENT BY 1")
		})

		t.Run("a new enum value is one statement", func(t *testing.T) {
			must[Ticket2DB](p, "diff", "add_pending")

			require.Contains(
				t,
				p.read("versions/0002_add_pending.up.sql"),
				`ALTER TYPE "ticket_status" ADD VALUE IF NOT EXISTS 'pending'`,
			)

			must[Ticket2DB](p, "up")
			require.Contains(t, must[Ticket2DB](p, "drift"), "No drift")
		})

		t.Run("removing a value needs a person, and diff says so", func(t *testing.T) {
			out := must[Ticket3DB](p, "diff", "remove_closed", "--dry-run")
			require.Contains(t, out, "PostgreSQL cannot remove value closed")
		})
	})
}

// EnsureCreated, for tests and prototypes, makes the enums and sequences the
// models use before their tables, and leaves ones already there alone.
func TestEnsureCreatedWithEnumsAndSequences(t *testing.T) {
	each(t, func(t *testing.T, p *project) {
		db := open[TicketDB](p)
		ctx := t.Context()

		require.NoError(t, db.EnsureCreated(ctx))

		ticket := &Ticket{Code: "abcd"}
		require.NoError(t, db.Tickets.Insert(ctx, ticket))
		require.EqualValues(t, 1000, ticket.Number)
		require.Equal(t, "open", ticket.Status)

		require.NoError(t, db.EnsureCreated(ctx), "a second time finds them all")

		t.Run("only what is missing", func(t *testing.T) {
			p.sql("DROP TABLE tickets")
			p.sql("DROP SEQUENCE ticket_numbers")

			require.NoError(t, db.EnsureCreated(ctx), "the enum is still there")

			again := &Ticket{Code: "efgh"}
			require.NoError(t, db.Tickets.Insert(ctx, again))
			require.EqualValues(t, 1000, again.Number, "a new sequence")
		})
	})
}

// LineItem has a foreign key of two columns, which relations cannot express.
type LineItem struct {
	ID      int64
	OrderID int64
	Region  string
}

func (LineItem) Configure(b *ant.ModelBuilder[LineItem]) {
	b.ForeignKey(ant.F(&LineItems.OrderID), ant.F(&LineItems.Region)).
		References(ant.F(&ShardedOrders.ID), ant.F(&ShardedOrders.Region)).
		OnDelete(ant.Cascade)
}

// ShardedOrder has a composite key.
type ShardedOrder struct {
	ID     int64
	Region string
}

func (ShardedOrder) Configure(b *ant.ModelBuilder[ShardedOrder]) {
	b.CompositeKey(ant.F(&ShardedOrders.ID), ant.F(&ShardedOrders.Region))
}

// Probes.
var (
	LineItems     = ant.Fields[LineItem]()
	ShardedOrders = ant.Fields[ShardedOrder]()
)

// ShardDB holds them.
type ShardDB struct {
	ant.Context
	Orders ant.Table[ShardedOrder]
	Items  ant.Table[LineItem]
}

func TestCompositeForeignKey(t *testing.T) {
	each(t, func(t *testing.T, p *project) {
		must[ShardDB](p, "diff", "start")

		require.Contains(
			t,
			p.read("versions/0001_start.up.sql"),
			`FOREIGN KEY ("order_id", "region") REFERENCES "sharded_orders" ("id", "region") ON DELETE CASCADE`,
		)

		must[ShardDB](p, "up")
		require.Contains(t, must[ShardDB](p, "drift"), "No drift")
	})
}

// Reading is a row in a table partitioned by month.
type Reading struct {
	ID    int64
	At    time.Time
	Value float64
}

func (Reading) Configure(b *ant.ModelBuilder[Reading]) {
	b.Key(&Readings.ID).NotGenerated()
	b.CompositeKey(ant.F(&Readings.ID), ant.F(&Readings.At))
	b.PartitionByRange(ant.F(&Readings.At))
}

// Readings is Reading's probe.
var Readings = ant.Fields[Reading]()

// PartitionDB holds it.
type PartitionDB struct {
	ant.Context
	Readings ant.Table[Reading]
}

// Stamp is a row in a table partitioned by month on a timestamp without time
// zone, whose bounds PostgreSQL prints without a zone.
type Stamp struct {
	ID int64
	At time.Time
}

func (Stamp) Configure(b *ant.ModelBuilder[Stamp]) {
	b.Key(&Stamps.ID).NotGenerated()
	b.CompositeKey(ant.F(&Stamps.ID), ant.F(&Stamps.At))
	b.Column(&Stamps.At).Type("timestamp")
	b.PartitionByRange(ant.F(&Stamps.At))
}

// Stamps is Stamp's probe.
var Stamps = ant.Fields[Stamp]()

// StampDB holds it.
type StampDB struct {
	ant.Context
	Stamps ant.Table[Stamp]
}

// Day is a row in a table partitioned by month on a date column, whose bounds
// PostgreSQL prints as a plain date.
type Day struct {
	ID int64
	At time.Time
}

func (Day) Configure(b *ant.ModelBuilder[Day]) {
	b.Key(&Days.ID).NotGenerated()
	b.CompositeKey(ant.F(&Days.ID), ant.F(&Days.At))
	b.Column(&Days.At).Type("date")
	b.PartitionByRange(ant.F(&Days.At))
}

// Days is Day's probe.
var Days = ant.Fields[Day]()

// DayDB holds it.
type DayDB struct {
	ant.Context
	Days ant.Table[Day]
}

// Migrations create the partitioned parent, and postgres.Partitions keeps its
// months, which is routine work rather than a schema change.
func TestPartitions(t *testing.T) {
	each(t, func(t *testing.T, p *project) {
		ctx := t.Context()

		must[PartitionDB](p, "diff", "start")
		require.Contains(t, p.read("versions/0001_start.up.sql"), `PARTITION BY RANGE ("at")`)
		must[PartitionDB](p, "up")
		db := open[PartitionDB](p)
		months, err := postgres.MonthlyPartitions[Reading](db)
		require.NoError(t, err)

		start := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
		created, err := months.Ensure(ctx, start, 2)
		require.NoError(t, err)

		require.Equal(
			t,
			[]string{"readings_2026_01", "readings_2026_02", "readings_2026_03"},
			created,
		)

		t.Run("Ensure is safe to repeat", func(t *testing.T) {
			created, err := months.Ensure(ctx, start, 2)
			require.NoError(t, err)
			require.Empty(t, created)
		})

		t.Run("rows land in their month", func(t *testing.T) {
			require.NoError(t, db.Readings.Insert(ctx,
				&Reading{ID: 1, At: start, Value: 1},
				&Reading{ID: 2, At: start.AddDate(0, 1, 0), Value: 2}))

			require.Equal(t, 1, scalar[int](p, "SELECT count(*) FROM readings_2026_01"))
		})

		t.Run("drift looks at the parent, and ignores the months", func(t *testing.T) {
			require.Contains(t, must[PartitionDB](p, "drift"), "No drift")
		})

		t.Run("DropBefore removes whole months only", func(t *testing.T) {
			cutoff := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
			dropped, err := months.DropBefore(ctx, cutoff)
			require.NoError(t, err)
			require.Equal(t, []string{"readings_2026_01"}, dropped)

			left, err := months.List(ctx)
			require.NoError(t, err)
			require.Len(t, left, 2)
			require.Equal(t, 1, scalar[int](p, "SELECT count(*) FROM readings"))
		})
	})
}

// A month of a partitioned table need not be named table_YYYY_MM. A DBA may
// have made it before ant managed the table, and Pagila's are payment_p2022_01.
// What makes a partition a month is its bounds, so List, Ensure and DropBefore
// go by those, not by the name (#196).
func TestPartitionsNamedBySomethingElse(t *testing.T) {
	each(t, func(t *testing.T, p *project) {
		ctx := t.Context()

		must[PartitionDB](p, "diff", "start")
		must[PartitionDB](p, "up")

		db := open[PartitionDB](p)
		months, err := postgres.MonthlyPartitions[Reading](db)
		require.NoError(t, err)

		p.sql(`CREATE TABLE readings_apr26 PARTITION OF readings
			FOR VALUES FROM ('2026-04-01') TO ('2026-05-01')`)

		t.Run("List reads the month from the bounds", func(t *testing.T) {
			got, err := months.List(ctx)
			require.NoError(t, err)
			require.Len(t, got, 1)
			require.Equal(t, "readings_apr26", got[0].Name)
			require.Equal(t, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), got[0].Month)
		})

		t.Run("Ensure sees the month is taken", func(t *testing.T) {
			created, err := months.Ensure(ctx, time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), 0)
			require.NoError(t, err)
			require.Empty(t, created)
		})

		t.Run("DropBefore reaches it", func(t *testing.T) {
			dropped, err := months.DropBefore(ctx, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC))
			require.NoError(t, err)
			require.Equal(t, []string{"readings_apr26"}, dropped)
		})
	})
}

// A table partitioned on a timestamp without time zone prints its months'
// bounds without a zone, as '2026-04-01 00:00:00'. Such a table's months are
// months too: List reads them, and Ensure and DropBefore act on them (#196).
func TestPartitionsOnATimestampWithoutZone(t *testing.T) {
	each(t, func(t *testing.T, p *project) {
		ctx := t.Context()

		must[StampDB](p, "diff", "start")
		must[StampDB](p, "up")

		db := open[StampDB](p)
		months, err := postgres.MonthlyPartitions[Stamp](db)
		require.NoError(t, err)

		p.sql(`CREATE TABLE stamps_apr26 PARTITION OF stamps
			FOR VALUES FROM ('2026-04-01') TO ('2026-05-01')`)

		got, err := months.List(ctx)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, "stamps_apr26", got[0].Name)
		require.Equal(t, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), got[0].Month)

		created, err := months.Ensure(ctx, time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), 0)
		require.NoError(t, err)
		require.Empty(t, created)

		dropped, err := months.DropBefore(ctx, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		require.Equal(t, []string{"stamps_apr26"}, dropped)
	})
}

// A table partitioned on a date prints its months' bounds as a plain date, as
// '2026-04-01', and those months are months too (#196).
func TestPartitionsOnADate(t *testing.T) {
	each(t, func(t *testing.T, p *project) {
		ctx := t.Context()

		must[DayDB](p, "diff", "start")
		must[DayDB](p, "up")

		db := open[DayDB](p)
		months, err := postgres.MonthlyPartitions[Day](db)
		require.NoError(t, err)

		p.sql(`CREATE TABLE days_apr26 PARTITION OF days
			FOR VALUES FROM ('2026-04-01') TO ('2026-05-01')`)

		got, err := months.List(ctx)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, "days_apr26", got[0].Name)
		require.Equal(t, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), got[0].Month)

		dropped, err := months.DropBefore(ctx, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		require.Equal(t, []string{"days_apr26"}, dropped)

		// A date column takes the zone the CREATE names, and the bounds it
		// prints afterward read back as the same month.
		created, err := months.Ensure(ctx, time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC), 0)
		require.NoError(t, err)
		require.Equal(t, []string{"days_2026_06"}, created)

		got, err = months.List(ctx)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), got[0].Month)
	})
}

// A partition whose bounds are not a whole month is not a month of the table,
// so List leaves it alone and DropBefore does not remove it.
func TestPartitionsOfOtherSpansAreLeftAlone(t *testing.T) {
	each(t, func(t *testing.T, p *project) {
		ctx := t.Context()

		must[PartitionDB](p, "diff", "start")
		must[PartitionDB](p, "up")

		db := open[PartitionDB](p)
		months, err := postgres.MonthlyPartitions[Reading](db)
		require.NoError(t, err)

		p.sql(`CREATE TABLE readings_half PARTITION OF readings
			FOR VALUES FROM ('2026-06-01') TO ('2026-06-16')`)

		got, err := months.List(ctx)
		require.NoError(t, err)
		require.Empty(t, got)

		dropped, err := months.DropBefore(ctx, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		require.Empty(t, dropped)
	})
}

// A partition named what Ensure would name a month, but spanning something
// else, cannot be replaced: CREATE TABLE IF NOT EXISTS would skip it. Ensure
// says so rather than reporting a month created that is not there.
func TestPartitionsRefuseANameTakenByAnotherSpan(t *testing.T) {
	each(t, func(t *testing.T, p *project) {
		ctx := t.Context()

		must[PartitionDB](p, "diff", "start")
		must[PartitionDB](p, "up")

		db := open[PartitionDB](p)
		months, err := postgres.MonthlyPartitions[Reading](db)
		require.NoError(t, err)

		p.sql(`CREATE TABLE readings_2026_04 PARTITION OF readings
			FOR VALUES FROM ('2026-04-01') TO ('2026-04-16')`)

		_, err = months.Ensure(ctx, time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), 0)
		require.ErrorContains(
			t,
			err,
			"readings_2026_04 is taken by a partition that is not the month of 2026-04",
		)

		got, err := months.List(ctx)
		require.NoError(t, err)
		require.Empty(t, got)
	})
}

// An enum and a sequence whose names have capitals are created quoted, so the
// columns that use them quote them too: PostgreSQL folds a name without quotes
// to lower case, and the CREATE TABLE looked for a type that was not there
// (#100).
type Shout struct {
	ID     int64
	Number int64
	Status string
}

func (Shout) Configure(b *ant.ModelBuilder[Shout]) {
	b.Column(&Shouts.Number).Sequence("ShoutNumbers", ant.StartAt(7))
	b.Column(&Shouts.Status).Enum("Shout Status", "Loud", "quiet").Default("'Loud'")
}

var Shouts = ant.Fields[Shout]()

type ShoutDB struct {
	ant.Context
	Shouts ant.Table[Shout]
}

type Shout2 struct {
	ID     int64
	Number int64
	Status string
	Mood   string
}

func (Shout2) Configure(b *ant.ModelBuilder[Shout2]) {
	b.Table("shouts")
	b.Column(&Shout2s.Number).Sequence("ShoutNumbers", ant.StartAt(7))
	b.Column(&Shout2s.Status).Enum("Shout Status", "Loud", "quiet").Default("'Loud'")
	b.Column(&Shout2s.Mood).Enum("Mood", "Calm", "angry")
}

var Shout2s = ant.Fields[Shout2]()

type Shout2DB struct {
	ant.Context
	Shouts ant.Table[Shout2]
}

func TestEnumAndSequenceNamesAreQuoted(t *testing.T) {
	each(t, func(t *testing.T, p *project) {
		must[ShoutDB](p, "diff", "start")
		text := p.read("versions/0001_start.up.sql")
		require.Contains(t, text, `"status" "Shout Status" NOT NULL DEFAULT 'Loud'`)
		require.Contains(t, text, `nextval('"ShoutNumbers"')`)
		must[ShoutDB](p, "up")

		shout := &Shout{}
		require.NoError(t, open[ShoutDB](p).Shouts.Insert(t.Context(), shout))
		require.EqualValues(t, 7, shout.Number)
		require.Equal(t, "Loud", shout.Status)
		require.Contains(t, must[ShoutDB](p, "drift"), "No drift")

		t.Run("a new column of such an enum fills existing rows from it", func(t *testing.T) {
			must[Shout2DB](p, "diff", "add_mood")

			require.Contains(
				t,
				p.read("versions/0002_add_mood.up.sql"),
				`"mood" "Mood" NOT NULL DEFAULT 'Calm'`,
			)

			must[Shout2DB](p, "up")
			require.Contains(t, must[Shout2DB](p, "drift"), "No drift")
			require.Contains(t, must[Shout2DB](p, "check"), "OK")
		})
	})
}

// Reading2 is Reading partitioned another way, and Reading3 not at all.
type Reading2 struct {
	ID    int64
	At    time.Time
	Value float64
}

func (Reading2) Configure(b *ant.ModelBuilder[Reading2]) {
	b.Table("readings")
	b.Key(&Reading2s.ID).NotGenerated()
	b.CompositeKey(ant.F(&Reading2s.ID), ant.F(&Reading2s.At))
	b.PartitionByList(ant.F(&Reading2s.At))
}

var Reading2s = ant.Fields[Reading2]()

type Partition2DB struct {
	ant.Context
	Readings ant.Table[Reading2]
}

type Reading3 struct {
	ID    int64
	At    time.Time
	Value float64
}

func (Reading3) Configure(b *ant.ModelBuilder[Reading3]) {
	b.Table("readings")
	b.Key(&Reading3s.ID).NotGenerated()
	b.CompositeKey(ant.F(&Reading3s.ID), ant.F(&Reading3s.At))
}

var Reading3s = ant.Fields[Reading3]()

type Partition3DB struct {
	ant.Context
	Readings ant.Table[Reading3]
}

// How a table is partitioned can't change in place, and it was never compared,
// so check, diff and drift said the database matched the models (#96). Each now
// names the change, and the file diff writes says to make it by hand.
func TestRepartitioningIsAChange(t *testing.T) {
	each(t, func(t *testing.T, p *project) {
		must[PartitionDB](p, "diff", "start")
		must[PartitionDB](p, "up")

		for _, c := range []struct {
			name string
			run  func(args ...string) (string, error)
			want string
		}{
			{
				"by another method",
				func(args ...string) (string, error) { return run[Partition2DB](p, args...) },
				`repartition readings: partitioned by RANGE (at) to partitioned by LIST (at)`,
			},
			{
				"no longer",
				func(args ...string) (string, error) { return run[Partition3DB](p, args...) },
				`repartition readings: partitioned by RANGE (at) to not partitioned`,
			},
		} {
			t.Run(c.name, func(t *testing.T) {
				_, err := c.run("check")
				require.ErrorIs(t, err, migrate.ErrPendingChanges)
				require.ErrorContains(t, err, c.want)

				out, err := c.run("diff", "repartition", "--dry-run")
				require.NoError(t, err)
				require.Contains(t, out, "PostgreSQL cannot change how readings is partitioned")
			})
		}

		require.Contains(
			t,
			must[PartitionDB](p, "drift"),
			"No drift",
			"the same partitioning is no change",
		)

		t.Run(
			"drift sees a table partitioned otherwise than the migrations say",
			func(t *testing.T) {
				p.sql("DROP TABLE readings")

				p.sql(`CREATE TABLE readings (id bigint NOT NULL, at timestamptz NOT NULL,
				value double precision NOT NULL, PRIMARY KEY (id, at))`)

				out, err := run[PartitionDB](p, "drift")
				require.ErrorIs(t, err, migrate.ErrDrift)

				require.Contains(
					t,
					out,
					"repartition readings: not partitioned to partitioned by RANGE (at)",
				)
			},
		)
	})
}
