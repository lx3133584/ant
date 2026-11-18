package querying

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/tests/features/fixture"
)

// namedNames is a slice of a named string type, which the driver cannot encode
// as an array on its own.
type namedNames []string

// namedStatuses stands in for a slice of an enum element type.
type namedStatuses []CampaignStatus

// CampaignStatus is the named element type from the report.
type CampaignStatus string

// Raw SQL can use ? instead of $1, and a slice fills an IN list.
func TestRawArguments(t *testing.T) {
	fixture.Run(t, func(t *testing.T, db *fixture.AppDB) {
		fixture.Seed(t, db)
		ctx := t.Context()
		ages := []int{17, 30}
		namedNames := namedNames{"Alice", "Bob"}

		t.Run("? is a placeholder", func(t *testing.T) {
			got, err := ant.Raw[string](
				db,
				`SELECT name FROM users WHERE age > ? AND name <> ?`,
				18,
				"Alice",
			).All(ctx)
			require.NoError(t, err)
			require.Equal(t, []string{"Carol"}, got)
		})

		t.Run("a slice fills IN (?)", func(t *testing.T) {
			got, err := ant.Raw[string](
				db,
				`SELECT name FROM users WHERE age IN (?) ORDER BY name`,
				ages,
			).All(ctx)
			require.NoError(t, err)
			require.Equal(t, []string{"Alice", "Bob"}, got)
		})

		t.Run("a slice fills IN (@name)", func(t *testing.T) {
			got, err := ant.Raw[string](
				db,
				`SELECT name FROM users WHERE age NOT IN (@ages)`,
				sql.Named("ages", ages),
			).All(ctx)
			require.NoError(t, err)
			require.Equal(t, []string{"Carol"}, got)
		})

		t.Run("an empty slice matches nothing", func(t *testing.T) {
			got, err := ant.Raw[string](
				db,
				`SELECT name FROM users WHERE age IN (?)`,
				[]int{},
			).All(ctx)
			require.NoError(t, err)
			require.Empty(t, got)
		})

		// SQL rule: NOT IN (NULL) is never true. ant.NotIn with no values
		// differs, and matches everything.
		t.Run("an empty slice in NOT IN also matches nothing", func(t *testing.T) {
			got, err := ant.Raw[string](
				db,
				`SELECT name FROM users WHERE age NOT IN (?)`,
				[]int{},
			).All(ctx)
			require.NoError(t, err)
			require.Empty(t, got)
		})

		t.Run("outside IN, a slice is one array value", func(t *testing.T) {
			got, err := ant.Raw[string](
				db,
				`SELECT name FROM users WHERE age = ANY($1) ORDER BY name`,
				ages,
			).All(ctx)
			require.NoError(t, err)
			require.Equal(t, []string{"Alice", "Bob"}, got)

			got, err = ant.Raw[string](
				db,
				`SELECT name FROM users WHERE age = ANY(@ages) ORDER BY name`,
				sql.Named("ages", ages),
			).All(ctx)
			require.NoError(t, err)
			require.Equal(t, []string{"Alice", "Bob"}, got)
		})

		t.Run("a slice of a named type is one array value", func(t *testing.T) {
			got, err := ant.Raw[string](
				db,
				`SELECT name FROM users WHERE name = ANY(?) ORDER BY name`,
				namedNames,
			).All(ctx)
			require.NoError(t, err)
			require.Equal(t, []string{"Alice", "Bob"}, got)
		})

		t.Run("a slice of a named type bound to an enum array", func(t *testing.T) {
			_, err := ant.Raw[any](db, `CREATE TYPE campaign_status AS ENUM ('draft', 'live')`).Exec(ctx)
			require.NoError(t, err)
			_, err = ant.Raw[any](db, `CREATE TEMP TABLE campaigns (status campaign_status NOT NULL)`).Exec(ctx)
			require.NoError(t, err)
			_, err = ant.Raw[any](db, `INSERT INTO campaigns (status) VALUES ('draft')`).Exec(ctx)
			require.NoError(t, err)

			for _, arg := range []any{
				namedStatuses{CampaignStatus("draft")},
				[]CampaignStatus{"draft"},
			} {
				for _, query := range []string{
					`SELECT count(*) FROM campaigns WHERE status = ANY(?)`,
					`SELECT count(*) FROM campaigns WHERE status = ANY($1)`,
				} {
					got, err := ant.Raw[int64](db, query, arg).First(ctx)
					require.NoError(t, err, "%s with %T", query, arg)
					require.Equal(t, int64(1), *got)
				}
			}

			found, err := db.Users.Where(
				ant.SQLCond(`name = ANY(?)`, namedNames),
			).Count(ctx)
			require.NoError(t, err, "a condition binds it as one array too")
			require.Equal(t, int64(2), found)
		})

		t.Run("the jsonb ? operator still works", func(t *testing.T) {
			got, err := ant.Raw[bool](db, `SELECT '{"a": 1}'::jsonb ? $1`, "a").First(ctx)
			require.NoError(t, err)
			require.True(t, *got)

			got, err = ant.Raw[bool](db, `SELECT '{"a": 1}'::jsonb ? 'b'`).First(ctx)
			require.NoError(t, err)
			require.False(t, *got)
		})
	})
}
