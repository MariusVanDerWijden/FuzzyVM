package main

import (
	"bytes"
	"crypto/sha256"
	"flag"
	"fmt"
	"log"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/MariusVanDerWijden/FuzzyVM/fuzzer"
	"github.com/MariusVanDerWijden/FuzzyVM/generator"
	"github.com/cockroachdb/pebble"
)

var dbFile = "fuzzyvm-db.pebble"

func main() {
	path := flag.String("db", dbFile, "path to the pebble database")
	flag.Parse()

	// Open read-only so the stats tool neither creates an empty db
	// nor contends with a running fuzzer for the directory lock.
	db, err := pebble.Open(*path, &pebble.Options{ReadOnly: true, ErrorIfNotExists: true})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	metrics := db.Metrics()
	fmt.Printf("Reading metrics for %v\n", *path)
	fmt.Printf("Estimated disk usage: %.2fM\n", float64(metrics.DiskSpaceUsage())/1024/1024)
	fmt.Printf("Key count: %v\n", countKeys(db))
}

func countKeys(db *pebble.DB) int {
	iter, err := db.NewIter(&pebble.IterOptions{})
	if err != nil {
		panic(err)
	}
	defer iter.Close()
	keys := 0
	for iter.First(); iter.Valid(); iter.Next() {
		keys++
	}
	return keys
}

func createDB(file string) (*pebbleDB, error) {
	pdb, err := pebble.Open(file, &pebble.Options{})
	if err != nil {
		return nil, err
	}
	return &pebbleDB{db: pdb}, nil
}

func makeKey(code []byte) []byte {
	hash := sha256.Sum256(code)
	return hash[:]
}

type db interface {
	Get(key []byte) ([]byte, error)
	Set(key, value []byte) error
	SetBatch(keys, values [][]byte) error
	Close() error
}

type pebbleDB struct {
	db *pebble.DB
}

func (db *pebbleDB) Get(key []byte) ([]byte, error) {
	val, closer, err := db.db.Get(key)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	// The slice returned by pebble is only valid until closer is closed.
	return bytes.Clone(val), nil
}

func (db *pebbleDB) Set(key, value []byte) error {
	return db.db.Set(key, value, pebble.NoSync)
}

func (db *pebbleDB) SetBatch(keys, values [][]byte) error {
	batch := db.db.NewBatch()
	defer batch.Close()
	for i, key := range keys {
		if err := batch.Set(key, values[i], nil); err != nil {
			return err
		}
	}
	return batch.Commit(pebble.NoSync)
}

func (db *pebbleDB) Close() error {
	return db.db.Close()
}

func hasCode(db db, code []byte) (bool, error) {
	if _, err := db.Get(makeKey(code)); err == pebble.ErrNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func putCode(db db, code []byte) error {
	return db.Set(makeKey(code), code)
}

func run(db db, input []byte) error {
	// Too little data destroys our performance and makes it hard for the generator
	if len(input) < 32 {
		return nil
	}
	f := filler.NewFiller(input)
	gst, bytecode := generator.GenerateProgram(f)
	if have, err := hasCode(db, bytecode); err != nil {
		return err
	} else if have {
		// already have this code in our db, skip
		return nil
	}
	_, minCode, err := fuzzer.MinimizeProgram(gst)
	if err != nil {
		// A program that fails to minimize is not worth stopping a campaign for.
		log.Printf("skipping program that failed to minimize: %v", err)
		return nil
	}
	if have, err := hasCode(db, minCode); err != nil {
		return err
	} else if have {
		// a different program already minimized to this code
		return putCode(db, bytecode)
	}
	// Store both codes atomically so a failure between the writes can't leave
	// the full code present (and thus skipped forever) without its minimized
	// counterpart.
	return db.SetBatch(
		[][]byte{makeKey(bytecode), makeKey(minCode)},
		[][]byte{bytecode, minCode},
	)
}
