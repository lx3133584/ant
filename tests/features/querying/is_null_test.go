package querying

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/tests/features/fixture"
)

// ant.IsNull and ant.IsNotNull test a nullable column.
func TestIsNull(t *testing.T) {
	fixture.Run(t, func(t *testing.T, db *fixture.AppDB) {
		fixture.Seed(t, db)
		ctx := t.Context()

		t.Run("IsNull", func(t *testing.T) {
			got, err := db.Users.Where(ant.IsNull(&Users.Bio)).OrderBy(&Users.Name).All(ctx)
			require.NoError(t, err)
			require.Equal(t, []string{"Alice", "Bob"}, fixture.Names(got))
		})

		t.Run("IsNotNull", func(t *testing.T) {
			got, err := db.Users.Where(ant.IsNotNull(&Users.Bio)).All(ctx)
			require.NoError(t, err)
			require.Equal(t, []string{"Carol"}, fixture.Names(got))
		})

		t.Run("a column that is never NULL matches nothing", func(t *testing.T) {
			got, err := db.Users.Where(ant.IsNull(&Users.Name)).All(ctx)
			require.NoError(t, err)
			require.Empty(t, got)
		})
	})
}

// A condition on a relation names the field, so a long chain can be bisected
// without guessing which one it was.
func TestRelationConditionNamesTheField(t *testing.T) {
	fixture.Run(t, func(t *testing.T, db *fixture.AppDB) {
		ctx := t.Context()

		_, err := db.Users.Where(
			ant.Eq(&Users.Name, "x"),
			ant.IsNull(&Users.Posts),
		).All(ctx)
		require.ErrorIs(t, err, ant.ErrNotColumn)
		require.ErrorContains(t, err, "User.Posts")
	})
}
