package developer_tooling

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/tests/features/fixture"
)

var Posts = fixture.Posts

// Crate and Lid are a pair of models for the two below to borrow from.
type Crate struct {
	ID    int64
	Label string
}

type Lid struct {
	ID      int64
	CrateID int64
	Crate   *Crate
}

var (
	Crates = ant.Fields[Crate]()
	Lids   = ant.Fields[Lid]()
)

// BorrowedKey configures its key with a field of Crate.
type BorrowedKey struct {
	ID   int64
	Code string
}

func (BorrowedKey) Configure(b *ant.ModelBuilder[BorrowedKey]) {
	//antvet:ignore another model's field, on purpose: the test checks the error
	b.Key(&Crates.Label)
}

// BorrowedRelation configures a relation of Lid as its own.
type BorrowedRelation struct {
	ID      int64
	CrateID int64
	Crate   *Crate
}

func (BorrowedRelation) Configure(b *ant.ModelBuilder[BorrowedRelation]) {
	//antvet:ignore another model's relation, on purpose: the test checks the error
	b.BelongsTo(&Lids.Crate)
}

// A field or a relation used with a model it does not belong to is named, with
// the model it was used with, wherever the mistake is made: a chain of calls is
// not bisected to find which one it was (#166).
func TestWrongModelNamesTheField(t *testing.T) {
	fixture.Run(t, func(t *testing.T, db *fixture.AppDB) {
		ctx := t.Context()
		alice, _, _ := fixture.Seed(t, db)

		t.Run("a column chosen from another model", func(t *testing.T) {
			//antvet:ignore another model's field, on purpose: the test checks the error
			_, err := db.Users.Column(&Posts.Title).All(ctx)
			require.ErrorIs(t, err, ant.ErrWrongModel)
			require.ErrorContains(t, err, "Post.Title in a query of User")
		})

		t.Run("a computed column of another model", func(t *testing.T) {
			//antvet:ignore another model's field, on purpose: the test checks the error
			_, err := db.Users.With(&Posts.Title, ant.SQL[string]("'x'")).All(ctx)
			require.ErrorIs(t, err, ant.ErrWrongModel)
			require.ErrorContains(t, err, "Post.Title in a query of User")
		})

		t.Run("a condition among others", func(t *testing.T) {
			_, err := db.Users.Where(
				ant.Eq(&Users.Name, "x"),
				//antvet:ignore another model's field, on purpose: the test checks the error
				ant.Eq(&Posts.Published, true),
				ant.Eq(&Users.Email, "y"),
			).All(ctx)
			require.ErrorIs(t, err, ant.ErrWrongModel)
			require.ErrorContains(t, err, "Post.Published in a query of User")
		})

		t.Run("an included relation of another model", func(t *testing.T) {
			//antvet:ignore another model's relation, on purpose: the test checks the error
			_, err := db.Posts.Include(&Users.Posts).All(ctx)
			require.ErrorIs(t, err, ant.ErrWrongModel)
			require.ErrorContains(t, err, "User.Posts in a query of Post")
		})

		t.Run("an assignment to another model's field", func(t *testing.T) {
			_, err := db.Users.Where(ant.Eq(&Users.ID, alice.ID)).
				//antvet:ignore another model's field, on purpose: the test checks the error
				Update(ctx, ant.Set(&Posts.Title, "x"))
			require.ErrorIs(t, err, ant.ErrWrongModel)
			require.ErrorContains(t, err, "Post.Title in a query of User")
		})

		t.Run("a recursion through another model's relation", func(t *testing.T) {
			//antvet:ignore another model's relation, on purpose: the test checks the error
			_, err := db.Users.Where().RecursiveOne(&Posts.User).All(ctx)
			require.ErrorIs(t, err, ant.ErrWrongModel)
			require.ErrorContains(t, err, "Post.User in a query of User")
		})

		t.Run("the related rows of another model's relation", func(t *testing.T) {
			//antvet:ignore another model's relation, on purpose: the test checks the error
			_, err := ant.RelatedOne(db, alice, &Posts.User).All(ctx)
			require.ErrorIs(t, err, ant.ErrWrongModel)
			require.ErrorContains(t, err, "Post.User is not a relation of User")
		})

		t.Run("a link through another model's relation", func(t *testing.T) {
			post := &fixture.Post{Base: ant.Base{ID: 1}}
			//antvet:ignore another model's relation, on purpose: the test checks the error
			err := ant.Link(ctx, db, post, &Users.Posts, &fixture.Post{Base: ant.Base{ID: 2}})
			require.ErrorIs(t, err, ant.ErrWrongModel)
			require.ErrorContains(t, err, "User.Posts is not a relation of Post")
		})

		t.Run("a bulk update of another model's field", func(t *testing.T) {
			//antvet:ignore another model's field, on purpose: the test checks the error
			_, err := db.Users.UpdateMany(ctx, []*fixture.User{alice}, ant.F(&Posts.Title))
			require.ErrorIs(t, err, ant.ErrWrongModel)
			require.ErrorContains(t, err, "Post.Title is not a field of User")
		})

		t.Run("a column where a relation goes", func(t *testing.T) {
			//antvet:ignore a column as a relation, on purpose: the test checks the error
			err := db.Users.LoadOne(ctx, &Users.Bio, alice)
			require.ErrorIs(t, err, ant.ErrNotRelation)
			require.ErrorContains(t, err, "User.Bio")
		})
	})

	t.Run("a key configured with another model's field", func(t *testing.T) {
		err := opened[struct {
			ant.Context
			T ant.Table[BorrowedKey]
		}](t)
		require.ErrorIs(t, err, ant.ErrWrongModel)
		require.ErrorContains(t, err, "Crate.Label is not a field of BorrowedKey")
	})

	t.Run("a relation configured from another model", func(t *testing.T) {
		err := opened[struct {
			ant.Context
			T ant.Table[BorrowedRelation]
		}](t)
		require.ErrorIs(t, err, ant.ErrWrongModel)
		require.ErrorContains(t, err, "Lid.Crate is not a relation of BorrowedRelation")
	})
}
