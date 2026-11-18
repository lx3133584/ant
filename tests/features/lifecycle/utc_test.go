package lifecycle

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/tests/features/fixture"
)

// Moment holds a time in every shape a field can take.
type Moment struct {
	ID        int64
	At        time.Time
	Maybe     *time.Time
	NullAt    sql.Null[time.Time]
	Legacy    sql.NullTime
	Ats       []time.Time
	Stamped   time.Time `ant:"default"` // filled by the database, read back on insert
	DeletedAt ant.DeletedAt
}

// Moments is Moment's probe.
var Moments = ant.Fields[Moment]()

// MomentsDB holds it.
type MomentsDB struct {
	ant.Context
	Moments ant.Table[Moment]
}

const momentsDDL = `CREATE TABLE moments (
	id BIGSERIAL PRIMARY KEY,
	at TIMESTAMPTZ NOT NULL,
	maybe TIMESTAMPTZ,
	null_at TIMESTAMPTZ,
	legacy TIMESTAMPTZ,
	ats TIMESTAMPTZ[] NOT NULL,
	stamped TIMESTAMPTZ NOT NULL DEFAULT now(),
	deleted_at TIMESTAMPTZ)`

// A time comes back in UTC, whatever zone the driver decodes it in: pgx gives
// TIMESTAMPTZ in the process's zone, so a row read back would otherwise differ
// from the one written, and a JSON API would render a create as ...Z and a get
// as +02:00. The instant does not move.
func TestTimesReadBackInUTC(t *testing.T) {
	at := time.Date(2026, 11, 14, 23, 29, 49, 978013000, time.UTC)

	inUTC := func(t *testing.T, name string, got time.Time) {
		t.Helper()
		require.Equal(t, time.UTC, got.Location(), "%s is held in UTC", name)
	}

	fixture.RunWith(t, momentsDDL, func(t *testing.T, db *MomentsDB) {
		ctx := t.Context()

		row := &Moment{
			At:     at,
			Maybe:  &at,
			NullAt: sql.Null[time.Time]{V: at, Valid: true},
			Legacy: sql.NullTime{Time: at, Valid: true},
			Ats:    []time.Time{at, at.Add(time.Hour)},
		}
		require.NoError(t, db.Moments.Insert(ctx, row))
		inUTC(t, "a time the insert read back", row.Stamped)

		back, err := db.Moments.NoTracking().Find(ctx, row.ID)
		require.NoError(t, err)
		require.Equal(t, at, back.At, "identical, not merely the same instant")
		inUTC(t, "a pointer's time", *back.Maybe)
		inUTC(t, "an sql.Null's time", back.NullAt.V)
		inUTC(t, "an sql.NullTime's time", back.Legacy.Time)
		require.Equal(t, row.Ats, back.Ats)

		for _, one := range back.Ats {
			inUTC(t, "each time of a slice", one)
		}

		inUTC(t, "a time set by the database", back.Stamped)

		require.NoError(t, db.Moments.DeleteByID(ctx, row.ID))
		deleted, err := db.Moments.WithDeleted().NoTracking().Find(ctx, row.ID)
		require.NoError(t, err)
		require.True(t, deleted.DeletedAt.Valid)
		inUTC(t, "DeletedAt", deleted.DeletedAt.V)

		one, err := ant.Raw[time.Time](db, `SELECT at FROM moments`).First(ctx)
		require.NoError(t, err)
		inUTC(t, "a time read as a single value", *one)
	})

	fixture.Run(t, func(t *testing.T, db *fixture.AppDB) {
		ctx := t.Context()
		user := &fixture.User{Name: "zone", Email: "zone@example.com"}
		require.NoError(t, db.Users.Insert(ctx, user))
		require.NoError(t, db.Posts.Insert(ctx, &fixture.Post{UserID: user.ID, Title: "t"}))

		posts, err := db.Posts.IncludeJoined(&fixture.Posts.User).All(ctx)
		require.NoError(t, err)
		require.Len(t, posts, 1)
		inUTC(t, "a joined query's own time", posts[0].CreatedAt)
		inUTC(t, "a joined include's time", posts[0].User.CreatedAt)

		joined, err := db.Posts.Join(&fixture.Posts.User).
			Select(func(_ *fixture.Post, u *fixture.User) time.Time { return u.CreatedAt }).
			All(ctx)
		require.NoError(t, err)
		require.Len(t, joined, 1)
		inUTC(t, "a time read through a join", joined[0])
	})
}

// A NULL is not a time, and a time.Time field cannot hold one: database/sql
// says so in its own words, naming the column, rather than ant giving a zero
// time. A field that may be NULL is a pointer or an sql.Null.
func TestANullTimeIsStillRefused(t *testing.T) {
	const ddl = `CREATE TABLE stamps (id BIGSERIAL PRIMARY KEY, at TIMESTAMPTZ NULL)`

	fixture.RunWith(t, ddl, func(t *testing.T, db *StampsDB) {
		ctx := t.Context()

		_, err := db.DB().ExecContext(ctx, `INSERT INTO stamps (at) VALUES (NULL)`)
		require.NoError(t, err)

		_, err = db.Stamps.NoTracking().First(ctx)
		require.ErrorContains(
			t,
			err,
			`unsupported Scan, storing driver.Value type <nil> into type *time.Time`,
		)

		require.ErrorContains(t, err, `name "at"`)
	})
}

// Stamp has a time that cannot hold NULL.
type Stamp struct {
	ID int64
	At time.Time
}

// StampsDB holds it.
type StampsDB struct {
	ant.Context
	Stamps ant.Table[Stamp]
}
