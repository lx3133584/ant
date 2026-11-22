package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/migrate"
)

// Payment adopts a serial key and names its expected sequence explicitly.
type Payment struct {
	ID        int64
	PaymentID int64
}

func (Payment) Configure(b *ant.ModelBuilder[Payment]) {
	b.Column(&Payments.PaymentID).Default("nextval('rental_rental_id_seq'::regclass)")
}

// PaymentDB holds Payment.
type PaymentDB struct {
	ant.Context
	Payments ant.Table[Payment]
}

// Payments is Payment's probe.
var Payments = ant.Fields[Payment]()

// PaymentWithoutCast is the same table with the model default written without a cast.
type PaymentWithoutCast struct {
	ID        int64
	PaymentID int64
}

func (PaymentWithoutCast) Configure(b *ant.ModelBuilder[PaymentWithoutCast]) {
	b.Table("payments")
	b.Column(&PaymentWithoutCasts.PaymentID).Default("nextval('rental_rental_id_seq')")
}

type PaymentWithoutCastDB struct {
	ant.Context
	Payments ant.Table[PaymentWithoutCast]
}

var PaymentWithoutCasts = ant.Fields[PaymentWithoutCast]()

func TestDriftComparesSerialSequence(t *testing.T) {
	each(t, func(t *testing.T, p *project) {
		p.sql("CREATE SEQUENCE payment_payment_id_seq")
		p.sql("CREATE SEQUENCE rental_rental_id_seq")

		p.sql(
			"CREATE TABLE payments (id bigserial PRIMARY KEY, payment_id bigint NOT NULL DEFAULT nextval('payment_payment_id_seq'))",
		)

		p.sql("ALTER SEQUENCE payment_payment_id_seq OWNED BY payments.payment_id")
		p.sql("ALTER SEQUENCE rental_rental_id_seq OWNED BY payments.payment_id")

		out, err := run[PaymentDB](p, "drift", "--sql")
		require.ErrorIs(t, err, migrate.ErrDrift)
		require.Contains(t, err.Error(), "1 difference", "the total includes the serial sequence")
		require.Contains(t, out, "payment_payment_id_seq")
		require.Contains(t, out, "rental_rental_id_seq")

		require.NotContains(
			t,
			out,
			"SET DEFAULT",
			"drift reports the sequence without generating a migration",
		)

		p.sql(
			"ALTER TABLE payments ALTER COLUMN payment_id SET DEFAULT nextval('rental_rental_id_seq')",
		)

		require.Contains(t, must[PaymentDB](p, "drift"), "No drift")

		require.Contains(
			t,
			must[PaymentWithoutCastDB](p, "drift"),
			"No drift",
			"the model may omit PostgreSQL's regclass cast",
		)
	})
}
