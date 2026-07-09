package main

import (
	"sync"
	"testing"
)

// The db is opened lazily inside the fuzz callback: with `go test -fuzz`, the
// coordinator process also executes the FuzzEVM body, so opening the db there
// would hold the pebble directory lock and starve the worker process. Only a
// single worker can run (-parallel=1), as workers are separate processes and
// would collide on the lock. The db is closed by process exit.
var (
	fuzzDB     db
	fuzzDBOnce sync.Once
)

func FuzzEVM(f *testing.F) {
	f.Fuzz(func(t *testing.T, a []byte) {
		fuzzDBOnce.Do(func() {
			db, err := createDB(dbFile)
			if err != nil {
				panic(err)
			}
			fuzzDB = db
		})
		if err := run(fuzzDB, a); err != nil {
			t.Fatal(err)
		}
	})
}

func TestEVM(t *testing.T) {
	db, err := createDB(dbFile)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	db, err = createDB(dbFile)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
}

func TestRepro(t *testing.T) {
	input := []byte("0¨\x99ž\xb8>\xb0]\xd17\b*\xe9ן:\xd1")
	db, err := createDB(dbFile)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := run(db, input); err != nil {
		t.Fatal(err)
	}
}
