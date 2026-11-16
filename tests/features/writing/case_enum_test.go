package writing

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/tests/features/fixture"
)

// Campaign is one row of the enum table.
type Campaign struct {
	ID     int64
	Status string
	Note   string
	SendAt *int64
}

// Configure declares the enum column, so ant knows its type.
func (Campaign) Configure(b *ant.ModelBuilder[Campaign]) {
	b.Table("campaigns")
	b.Column(&Campaigns.Status).Enum("campaign_status", "scheduled", "running")
}

// Campaigns is Campaign's probe.
var Campaigns = ant.Fields[Campaign]()

// CampaignDB holds it.
type CampaignDB struct {
	ant.Context
	Campaigns ant.Table[Campaign]
}

const campaignDDL = `CREATE TYPE campaign_status AS ENUM ('scheduled', 'running');
	CREATE TABLE campaigns (
		id BIGSERIAL PRIMARY KEY,
		status campaign_status NOT NULL,
		note TEXT NOT NULL DEFAULT '',
		send_at BIGINT
	)`

// A CASE assigned to an enum column needs its parameters cast to that enum:
// PostgreSQL resolves an unknown parameter inside CASE to text, and text does
// not assign to an enum column. ant knows the type from the model's Enum
// declaration, so the caller should not have to write the cast.
func TestUpdateCaseIntoEnumColumn(t *testing.T) {
	fixture.RunWith(t, campaignDDL, func(t *testing.T, db *CampaignDB) {
		ctx := t.Context()
		f := ant.Fields[Campaign]()

		sendAt := int64(0)
		require.NoError(t, db.Campaigns.Insert(ctx, &Campaign{Status: "scheduled", SendAt: &sendAt}))
		require.NoError(t, db.Campaigns.Insert(ctx, &Campaign{Status: "scheduled"}))

		t.Run("the enum column takes the CASE", func(t *testing.T) {
			next := ant.Case[string]().
				When(ant.IsNotNull(&f.SendAt), "scheduled").
				Else("running")

			n, err := db.Campaigns.
				Where(ant.Eq(&f.ID, int64(1))).
				Update(ctx, ant.SetExpr(&f.Status, next))
			require.NoError(t, err, "a CASE assigned to an enum column must bind without a manual cast")
			require.EqualValues(t, 1, n)

			got, err := db.Campaigns.NoTracking().Where(ant.Eq(&f.ID, int64(1))).First(ctx)
			require.NoError(t, err)
			require.Equal(t, "scheduled", got.Status)

			// The second row has no send_at, so the same shape takes the ELSE
			// branch and stores "running".
			n, err = db.Campaigns.
				Where(ant.Eq(&f.ID, int64(2))).
				Update(ctx, ant.SetExpr(&f.Status, ant.Case[string]().
					When(ant.IsNotNull(&f.SendAt), "scheduled").
					Else("running")))
			require.NoError(t, err)
			require.EqualValues(t, 1, n)

			got, err = db.Campaigns.NoTracking().Where(ant.Eq(&f.ID, int64(2))).First(ctx)
			require.NoError(t, err)
			require.Equal(t, "running", got.Status)
		})

		t.Run("a text column keeps its parameters uncast", func(t *testing.T) {
			note := ant.Case[string]().
				When(ant.IsNotNull(&f.SendAt), "has time").
				Else("no time")

			n, err := db.Campaigns.
				Where(ant.Eq(&f.ID, int64(1))).
				Update(ctx, ant.SetExpr(&f.Note, note))
			require.NoError(t, err)
			require.EqualValues(t, 1, n)

			got, err := db.Campaigns.NoTracking().Where(ant.Eq(&f.ID, int64(1))).First(ctx)
			require.NoError(t, err)
			require.Equal(t, "has time", got.Note)
		})
	})
}
