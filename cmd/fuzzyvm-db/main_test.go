package main

import (
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/MariusVanDerWijden/FuzzyVM/generator/precompiles"
)

// The db handle is created lazily inside the fuzz callback. Under `generate`,
// the parent process owns the single pebble handle (pebble locks its directory
// exclusively) and exposes it over a Unix socket, so each worker process
// connects to that instead of opening pebble itself. Run directly via
// `go test -fuzz` (no socket), the harness falls back to a private temporary
// pebble database.
var (
	fuzzDB     db
	fuzzDBOnce sync.Once
)

func FuzzEVM(f *testing.F) {
	f.Fuzz(func(t *testing.T, a []byte) {
		fuzzDBOnce.Do(func() {
			precompiles.WarmupKZG()
			// Connect to the server.
			if addr := socketAddr(); addr != "" {
				db, err := dialSocketDB(addr)
				if err != nil {
					panic(err)
				}
				fuzzDB = db
			} else {
				dir, err := os.MkdirTemp("", "fuzzyvm-fuzz-")
				if err != nil {
					panic(err)
				}
				db, err := createDB(dir + "/fuzz.pebble")
				if err != nil {
					panic(err)
				}
				fuzzDB = db
			}
		})
		if err := run(fuzzDB, a); err != nil {
			// A broken socket (e.g. the server shutting down) is an
			// infrastructure failure, not a bug in this input. Skip it so it
			// isn't recorded as a reproducible fuzz crash.
			if errors.Is(err, errSocket) {
				t.Skipf("socket unavailable: %v", err)
			}
			t.Fatal(err)
		}
	})
}

func TestEVM(t *testing.T) {
	path := t.TempDir() + "/test.pebble"
	db, err := createDB(path)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	db, err = createDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
}

func TestRepro(t *testing.T) {
	input := []byte("0¨\x99ž\xb8>\xb0]\xd17\b*\xe9ן:\xd1")
	db, err := createDB(t.TempDir() + "/test.pebble")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := run(db, input); err != nil {
		t.Fatal(err)
	}
}
