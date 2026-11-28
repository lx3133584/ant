package ids

import (
	"bytes"
	"encoding/hex"
	"regexp"
	"sort"
	"testing"
	"time"
)

func TestUUID(t *testing.T) {
	shape := regexp.MustCompile(
		`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`,
	)

	first := UUID()
	if !shape.MatchString(first) {
		t.Fatalf("bad UUID %s", first)
	}

	time.Sleep(2 * time.Millisecond)

	if later := UUID(); later <= first {
		t.Fatalf("%s does not sort after %s", later, first)
	}
}

// key stands for any named 16-byte type, such as uuid.UUID.
type key [16]byte

func TestUUIDv7(t *testing.T) {
	for range 1000 {
		id := UUIDv7[key]()

		if version := id[6] >> 4; version != 7 {
			t.Fatalf("%x has version %d, want 7", id, version)
		}

		if variant := id[8] >> 6; variant != 0b10 {
			t.Fatalf("%x has variant bits %02b, want 10", id, variant)
		}
	}

	first := UUIDv7[key]()

	time.Sleep(2 * time.Millisecond)

	if later := UUIDv7[key](); bytes.Compare(later[:], first[:]) <= 0 {
		t.Fatalf("%x does not sort after %x", later, first)
	}

	if a, b := UUIDv7[key](), UUIDv7[key](); a == b {
		t.Fatalf("%x came out twice", a)
	}
}

// A plain array is a 16-byte type too, and the text form is the same value: its
// timestamp is the one UUIDv7 writes.
func TestUUIDv7AndUUIDShareALayout(t *testing.T) {
	before := time.Now().UnixMilli()
	binary := UUIDv7[[16]byte]()
	text := UUID()
	after := time.Now().UnixMilli()

	stamp := func(b []byte) int64 {
		var ms int64

		for _, c := range b {
			ms = ms<<8 | int64(c)
		}

		return ms
	}

	if ms := stamp(binary[:6]); ms < before || ms > after {
		t.Fatalf("UUIDv7 carries time %d, outside %d..%d", ms, before, after)
	}

	head, err := hex.DecodeString(text[0:8] + text[9:13])
	if err != nil {
		t.Fatal(err)
	}

	if ms := stamp(head); ms < before || ms > after {
		t.Fatalf("UUID carries time %d, outside %d..%d", ms, before, after)
	}
}

func TestULID(t *testing.T) {
	first := ULID()
	if len(first) != 26 {
		t.Fatalf("bad ULID %s", first)
	}

	time.Sleep(2 * time.Millisecond)

	if later := ULID(); later <= first {
		t.Fatalf("%s does not sort after %s", later, first)
	}
}

func TestSnowflake(t *testing.T) {
	if _, err := NewSnowflake(1024); err == nil {
		t.Fatal("node out of range was accepted")
	}

	s, err := NewSnowflake(7)
	if err != nil {
		t.Fatal(err)
	}

	keys := make([]int64, 10000)
	seen := map[int64]bool{}

	for i := range keys {
		keys[i] = s.Next()

		if seen[keys[i]] {
			t.Fatalf("duplicate key %d", keys[i])
		}

		seen[keys[i]] = true

		if keys[i]>>12&1023 != 7 {
			t.Fatalf("key %d lost its node", keys[i])
		}
	}

	if !sort.SliceIsSorted(keys, func(i, j int) bool { return keys[i] < keys[j] }) {
		t.Fatal("keys do not sort in order")
	}
}
