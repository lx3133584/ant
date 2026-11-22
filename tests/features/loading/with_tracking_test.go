package loading

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/tests/features/fixture"
)

const withTrackingDDL = `
	CREATE TABLE writers (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL);
	CREATE TABLE articles (id BIGSERIAL PRIMARY KEY, writer_id BIGINT NOT NULL REFERENCES writers, views INT NOT NULL, live BOOL NOT NULL)`

func TestWithTracksStoredFields(t *testing.T) {
	fixture.RunTrackedWith(t, withTrackingDDL, func(t *testing.T, db *PressDB) {
		ctx := t.Context()
		ann := &Writer{Name: "Ann"}
		require.NoError(t, db.Writers.Insert(ctx, ann))
		require.NoError(t, db.Articles.Insert(ctx, &Article{WriterID: ann.ID}))

		got, err := db.Writers.With(&Writers.ArticleCount, ant.CountRelated(&Writers.Articles)).
			Find(ctx, ann.ID)
		require.NoError(t, err)
		require.EqualValues(t, 1, got.ArticleCount)

		got.Name = "Renamed"
		got.ArticleCount = 99

		require.True(t, fixture.HasChanges(t, db))
		require.NoError(t, db.SaveChanges(ctx))

		stored, err := db.Writers.NoTracking().Find(ctx, ann.ID)
		require.NoError(t, err)
		require.Equal(t, "Renamed", stored.Name)
		require.Zero(t, stored.ArticleCount)

		got.ArticleCount = 100

		require.False(t, fixture.HasChanges(t, db), "a computed value alone is not a change")
	})
}

type UUIDWriter struct {
	ID       uuid.UUID
	Name     string
	Articles []UUIDArticle

	ArticleCount int64 `ant:"computed"`
}

type UUIDArticle struct {
	ID           int64
	UUIDWriterID uuid.UUID
}

type UUIDPressDB struct {
	ant.Context
	Writers  ant.Table[UUIDWriter]
	Articles ant.Table[UUIDArticle]
}

var (
	UUIDWriters  = ant.Fields[UUIDWriter]()
	UUIDArticles = ant.Fields[UUIDArticle]()
)

func (UUIDWriter) Configure(b *ant.ModelBuilder[UUIDWriter]) {
	b.Table("uuid_writers")
}

func (UUIDArticle) Configure(b *ant.ModelBuilder[UUIDArticle]) {
	b.Table("uuid_articles")
}

func TestWithTracksStoredFieldsWithUUIDKey(t *testing.T) {
	ddl := `
		CREATE TABLE uuid_writers (id UUID PRIMARY KEY, name TEXT NOT NULL);
		CREATE TABLE uuid_articles (id BIGSERIAL PRIMARY KEY, uuid_writer_id UUID NOT NULL REFERENCES uuid_writers)`
	fixture.RunTrackedWith(t, ddl, func(t *testing.T, db *UUIDPressDB) {
		ctx := t.Context()
		ann := &UUIDWriter{ID: uuid.New(), Name: "Ann"}
		require.NoError(t, db.Writers.Insert(ctx, ann))
		require.NoError(t, db.Articles.Insert(ctx, &UUIDArticle{UUIDWriterID: ann.ID}))

		got, err := db.Writers.With(&UUIDWriters.ArticleCount, ant.CountRelated(&UUIDWriters.Articles)).
			Find(ctx, ann.ID)
		require.NoError(t, err)
		require.EqualValues(t, 1, got.ArticleCount)

		got.Name = "Renamed"
		got.ArticleCount = 99

		require.True(t, fixture.HasChanges(t, db))
		require.NoError(t, db.SaveChanges(ctx))

		stored, err := db.Writers.NoTracking().Find(ctx, ann.ID)
		require.NoError(t, err)
		require.Equal(t, "Renamed", stored.Name)
		require.Zero(t, stored.ArticleCount)

		got.ArticleCount = 100

		require.False(t, fixture.HasChanges(t, db), "a computed value alone is not a change")
	})
}
