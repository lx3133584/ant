package keys_and_identity

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/ids"
	"github.com/skudra-go/ant/internal/testdb"
	"github.com/skudra-go/ant/tests/features/fixture"
)

// ids.UUIDv7 makes a time-ordered UUID of any 16-byte type, so a uuid.UUID key
// needs no wrapper around uuid.NewV7 and gets a native uuid column.
type Receipt struct {
	ID    uuid.UUID
	Total int
}

func (Receipt) Configure(b *ant.ModelBuilder[Receipt]) {
	ant.GenerateKey(b, &Receipts.ID, ids.UUIDv7[uuid.UUID])
}

// Memo's key is the same UUID as text. A string key is a text column, whatever
// fills it: 37 bytes a key, and any string is accepted as one.
type Memo struct {
	ID   string
	Body string
}

func (Memo) Configure(b *ant.ModelBuilder[Memo]) { ant.GenerateKey(b, &Memos.ID, ids.UUID) }

var (
	Receipts = ant.Fields[Receipt]()
	Memos    = ant.Fields[Memo]()
)

type LedgerDB struct {
	ant.Context
	Receipts ant.Table[Receipt]
	Memos    ant.Table[Memo]
}

func TestUUIDv7Key(t *testing.T) {
	fixture.Each(t, func(t *testing.T, target testdb.Target) {
		db := fixture.Open[LedgerDB](t, target)
		ctx := t.Context()
		require.NoError(t, db.EnsureCreated(ctx))

		columnType := func(t *testing.T, table string) string {
			t.Helper()

			var got string

			err := target.DB.QueryRowContext(
				ctx,
				`SELECT format_type(atttypid, atttypmod) FROM pg_attribute
				 WHERE attrelid = $1::regclass AND attname = 'id'`,
				table,
			).Scan(&got)
			require.NoError(t, err)

			return got
		}

		t.Run("a uuid.UUID key is a uuid column", func(t *testing.T) {
			require.Equal(t, "uuid", columnType(t, "receipts"))
		})

		t.Run("a string key is a text column", func(t *testing.T) {
			require.Equal(t, "text", columnType(t, "memos"))
		})

		t.Run("the key is a version 7 UUID", func(t *testing.T) {
			receipt := &Receipt{Total: 10}
			require.NoError(t, db.Receipts.Insert(ctx, receipt))
			require.Equal(t, uuid.Version(7), receipt.ID.Version())
			require.Equal(t, uuid.RFC4122, receipt.ID.Variant())

			found, err := db.Receipts.Find(ctx, receipt.ID)
			require.NoError(t, err)
			require.Equal(t, 10, found.Total)
		})

		t.Run("a later key sorts after an earlier one", func(t *testing.T) {
			first := &Receipt{Total: 1}
			require.NoError(t, db.Receipts.Insert(ctx, first))

			time.Sleep(2 * time.Millisecond)

			later := &Receipt{Total: 2}
			require.NoError(t, db.Receipts.Insert(ctx, later))
			require.Positive(t, bytes.Compare(later.ID[:], first.ID[:]))
		})

		t.Run("a key the application gives is kept", func(t *testing.T) {
			id := uuid.MustParse("0192f3a4-7c3d-7b1e-8f00-1234567890ab")
			require.NoError(t, db.Receipts.Insert(ctx, &Receipt{ID: id, Total: 3}))

			found, err := db.Receipts.Find(ctx, id)
			require.NoError(t, err)
			require.Equal(t, 3, found.Total)
		})
	})
}
