package lifecycle

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/tests/features/fixture"
	"github.com/skudra-go/ant/validate"
)

// Signup has rules on its fields.
type Signup struct {
	ID    int64
	Name  string
	Email string
	Age   int
}

func (Signup) Configure(b *ant.ModelBuilder[Signup]) {
	b.Validate(&Signups.Name, validate.All(validate.Required[string](), validate.MaxLength(5)))
	b.Validate(&Signups.Email, validate.Email)
	b.Validate(&Signups.Age, validate.Range(0, 150))
}

// Signups is Signup's probe.
var Signups = ant.Fields[Signup]()

// ValidDB holds it.
type ValidDB struct {
	ant.Context
	Signups ant.Table[Signup]
}

const validDDL = `CREATE TABLE signups (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL, email TEXT NOT NULL, age INT NOT NULL)`

// Rules check a field before a write, with an error that names it.
func TestFieldRules(t *testing.T) {
	fixture.RunTrackedWith(t, validDDL, func(t *testing.T, db *ValidDB) {
		ctx := t.Context()

		t.Run("a good row goes in", func(t *testing.T) {
			require.NoError(
				t,
				db.Signups.Insert(ctx, &Signup{Name: "Ada", Email: "a@example.com", Age: 30}),
			)
		})

		t.Run("a bad row does not, and the error names the field", func(t *testing.T) {
			err := db.Signups.Insert(ctx, &Signup{Name: "", Email: "a@example.com"})

			var invalid *ant.ValidationError
			require.ErrorAs(t, err, &invalid)
			require.Equal(t, "Name", invalid.Field)
			require.ErrorIs(t, err, validate.ErrRequired)
		})

		t.Run("updates are checked too", func(t *testing.T) {
			row, err := db.Signups.First(ctx)
			require.NoError(t, err)

			row.Age = 999
			err = db.SaveChanges(ctx)
			require.ErrorContains(t, err, "Age")

			row.Age = 31

			require.NoError(t, db.SaveChanges(ctx))
		})

		t.Run("ant.WithoutHooks skips them, as seeding wants", func(t *testing.T) {
			require.NoError(
				t,
				db.Signups.Insert(ant.WithoutHooks(ctx), &Signup{Name: "toolongname", Email: "x"}),
			)
		})
	})
}

// Enriched computes something from its relations, so it needs them loaded.
type Enriched struct {
	fixture.User
	Summary string `ant:"-"`
}

func (Enriched) Configure(b *ant.ModelBuilder[Enriched]) { b.Table("users") }

// AfterInclude runs once relations are in.
func (e *Enriched) AfterInclude(context.Context) error {
	e.Summary = e.Name + " is ready"

	if e.Name == "refuse" {
		return errors.New("refused")
	}

	return nil
}

// HookDB holds it.
type HookDB struct {
	ant.Context
	Users ant.Table[Enriched]
}

// AfterInclude runs after relations load, where AfterLoad runs before.
func TestAfterInclude(t *testing.T) {
	fixture.Each(t, func(t *testing.T, target fixture.Target) {
		ctx := t.Context()
		seed := fixture.Open[fixture.AppDB](t, target)
		fixture.Seed(t, seed)
		db := fixture.Open[HookDB](t, target)
		row, err := db.Users.First(ctx)
		require.NoError(t, err)
		require.Contains(t, row.Summary, "is ready")
	})
}

// Prune removes old rows for good, in batches, soft-deleted or not.
func TestPrune(t *testing.T) {
	fixture.Run(t, func(t *testing.T, db *fixture.AppDB) {
		ctx := t.Context()
		fixture.Seed(t, db)

		cutoff := time.Now().Add(time.Hour)
		removed, err := db.Users.Prune(ctx, ant.Lt(&fixture.Users.CreatedAt, cutoff))
		require.NoError(t, err)
		require.EqualValues(t, 3, removed)

		count, err := db.Users.Count(ctx)
		require.NoError(t, err)
		require.Zero(t, count)
	})
}

const uniqueSignupsDDL = `CREATE TABLE signups (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL, email TEXT NOT NULL UNIQUE, age INT NOT NULL)`

// Upsert checks the rules as Insert does.
func TestFieldRulesOnUpsert(t *testing.T) {
	fixture.RunWith(t, uniqueSignupsDDL, func(t *testing.T, db *ValidDB) {
		ctx := t.Context()

		_, err := db.Signups.Upsert(
			ctx,
			&Signup{Name: "", Email: "a@example.com"},
			ant.OnConflict(&Signups.Email).DoUpdate(ant.F(&Signups.Name)),
		)

		var invalid *ant.ValidationError
		require.ErrorAs(t, err, &invalid)
		require.Equal(t, "Name", invalid.Field)

		n, err := db.Signups.Count(ctx)
		require.NoError(t, err)
		require.Zero(t, n)
	})
}

// DefaultStatus is filled by the database when it is left zero.
type DefaultStatus string

const (
	DefaultDraft     DefaultStatus = "draft"
	DefaultPublished DefaultStatus = "published"
)

// DefaultedDoc validates Status and leaves its zero value to the database.
type DefaultedDoc struct {
	ID     int64
	Title  string
	Status DefaultStatus `ant:"default='draft'"`
}

func (DefaultedDoc) Configure(b *ant.ModelBuilder[DefaultedDoc]) {
	b.Validate(&DefaultedDocs.Status, validate.All(
		validate.Required[DefaultStatus](),
		validate.OneOf(DefaultDraft, DefaultPublished),
	))
}

var DefaultedDocs = ant.Fields[DefaultedDoc]()

// DefaultedDB holds the default-validated model.
type DefaultedDB struct {
	ant.Context
	DefaultedDocs ant.Table[DefaultedDoc]
}

const defaultedDDL = `
	CREATE TABLE defaulted_docs (
		id BIGSERIAL PRIMARY KEY,
		title TEXT NOT NULL UNIQUE,
		status TEXT NOT NULL DEFAULT 'draft'
	)`

func TestDefaultFieldRules(t *testing.T) {
	fixture.RunTrackedWith(t, defaultedDDL, func(t *testing.T, db *DefaultedDB) {
		ctx := t.Context()
		doc := &DefaultedDoc{Title: "zero"}
		require.NoError(t, db.DefaultedDocs.Insert(ctx, doc))
		require.NoError(t, db.ClearTracked(ctx))

		stored, err := db.DefaultedDocs.Find(ctx, doc.ID)
		require.NoError(t, err)
		require.Equal(t, DefaultDraft, stored.Status)

		stored.Status = ""
		err = db.SaveChanges(ctx)

		var invalid *ant.ValidationError
		require.ErrorAs(t, err, &invalid)

		require.Equal(
			t,
			"Status",
			invalid.Field,
			"an update writes the zero instead of taking the default",
		)

		stored.Status = DefaultPublished

		require.NoError(t, db.SaveChanges(ctx))

		err = db.DefaultedDocs.Insert(ctx, &DefaultedDoc{Title: "bad", Status: "invalid"})
		require.ErrorContains(t, err, "Status", "a nonzero value still has to satisfy the rule")

		require.NoError(t, db.DefaultedDocs.Insert(ctx,
			&DefaultedDoc{Title: "batch-one"},
			&DefaultedDoc{Title: "batch-two"}))

		count, err := db.DefaultedDocs.NoTracking().
			Where(ant.Eq(&DefaultedDocs.Status, DefaultDraft)).
			Count(ctx)
		require.NoError(t, err)
		require.EqualValues(t, 2, count)

		_, err = db.DefaultedDocs.Upsert(
			ctx,
			&DefaultedDoc{Title: "zero"},
			ant.OnConflict(&DefaultedDocs.Title).DoUpdate(ant.F(&DefaultedDocs.Status)),
		)
		require.ErrorContains(t, err, "Status", "an upsert's update branch can write the zero")
	})
}
