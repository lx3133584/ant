package lifecycle

import (
	"os"
	"testing"
	"time"
)

// Every test here runs in a zone other than UTC, so a time read back in the
// process's zone shows on any machine, including one whose zone is UTC.
func TestMain(m *testing.M) {
	time.Local = time.FixedZone("UTC+2", 2*60*60)

	os.Exit(m.Run())
}
