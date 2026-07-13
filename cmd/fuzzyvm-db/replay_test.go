package main

import (
	"bytes"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

// TestReplayCorpus is the coverage entry point invoked by `fuzzyvm-db replay`.
// It reads its parameters from the environment (see replay.go), opens the corpus
// read-only, and replays every stored bytecode through the EVM over a worker
// pool. It self-skips when no corpus is configured, so an ordinary
// `go test ./...` stays fast and green.
func TestReplayCorpus(t *testing.T) {
	dbPath := os.Getenv(replayDBEnv)
	if dbPath == "" {
		t.Skip("no corpus configured (set " + replayDBEnv + "); skipping replay")
	}
	limit := envInt(replayLimitEnv, 0)
	workers := envInt(replayWorkersEnv, runtime.NumCPU())
	if workers < 1 {
		workers = 1
	}

	db, err := openCorpus(dbPath)
	if err != nil {
		t.Fatalf("opening corpus %q: %v", dbPath, err)
	}
	defer db.Close()

	// Iteration is single-goroutine (the pebble iterator is not concurrent-safe
	// and its value is only valid until the next step), so one goroutine reads
	// and clones each code onto a channel that the workers drain.
	codes := make(chan []byte, workers*4)
	var (
		replayed atomic.Int64
		failed   atomic.Int64
	)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for code := range codes {
				if err := replayCode(code); err != nil {
					failed.Add(1)
				}
				replayed.Add(1)
			}
		}()
	}

	iterErr := forEachCode(db, limit, func(code []byte) error {
		codes <- bytes.Clone(code)
		return nil
	})
	close(codes)
	wg.Wait()

	if iterErr != nil {
		t.Fatalf("iterating corpus: %v", iterErr)
	}
	t.Logf("replayed %d codes (%d failed/panicked)", replayed.Load(), failed.Load())
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
