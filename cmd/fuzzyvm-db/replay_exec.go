package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/MariusVanDerWijden/FuzzyVM/generator"
	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/tests"
	"github.com/holiman/goevmlab/fuzzing"
)

// replaySeed is the fixed filler seed used to build the (throwaway) transaction
// and block environment that every stored bytecode is replayed under. It is all
// zero, which makes CreateGstMaker pick the maximum tx gas limit (the first
// filler byte 0 => gasLimit returns the generous default), zero value and zero
// calldata. Generous gas maximises how much of each bytecode executes, which is
// what we want when asking "what can this corpus reach at all". The corollary is
// that gas-metering/out-of-gas branches will legitimately show as uncovered;
// reaching those needs a separate constrained-gas pass.
var replaySeed = make([]byte, 128)

// replayTimeout bounds a single code's execution. Replay runs without go test's
// per-input watchdog, so a pathological program (e.g. a heavy precompile at max
// gas) could stall a worker and, with enough of them, the whole coverage run. 60s
// is far above any healthy code; one that exceeds it is skipped, not hung on.
const replayTimeout = 60 * time.Second

// replayCode replays one stored bytecode through the same state-test path the
// fuzzer uses (generator.CreateGstMaker -> tests.StateTest -> RunNoVerify), so
// the coverage it produces reflects the exact EVM surface FuzzyVM exercises. It
// runs the execution in a goroutine under replayTimeout and recovers from panics
// there, so neither a slow program nor a pathological one can abort or stall the
// replay.
func replayCode(code []byte) error {
	done := make(chan error, 1) // buffered: a timed-out goroutine can still send and exit
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic replaying code: %v", r)
			}
		}()
		gst := generator.CreateGstMaker(filler.NewFiller(replaySeed), code)
		done <- executeState(gst)
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(replayTimeout):
		return fmt.Errorf("replay exceeded %s", replayTimeout)
	}
}

// executeState mirrors fuzzer.MinimizeProgram's run path: marshal the GstMaker's
// single subtest to a geth tests.StateTest and RunNoVerify it. Unlike the fuzzer
// it closes the StateTestState it gets back — over a corpus of millions of codes
// the trie DB (and any snapshot goroutine) would otherwise leak.
func executeState(gst *fuzzing.GstMaker) error {
	name := ""
	gstPtr := gst.ToGeneralStateTest(name)
	sub := (*gstPtr)[name]

	data, err := json.Marshal(sub)
	if err != nil {
		return err
	}
	var stateTest tests.StateTest
	if err := json.Unmarshal(data, &stateTest); err != nil {
		return err
	}
	subtests := stateTest.Subtests()
	if len(subtests) == 0 {
		return fmt.Errorf("state test produced no subtests")
	}
	state, _, _, err := stateTest.RunNoVerify(subtests[0], vm.Config{}, false, rawdb.HashScheme)
	// Close is nil-safe (guards TrieDB != nil), so it is fine to call even after
	// an error return that left state at its zero value.
	state.Close()
	return err
}

// forEachCode iterates a read-only view of the corpus, invoking fn for each
// stored bytecode. If limit > 0 it stops after limit codes. The value slice
// pebble hands back is only valid until the next iteration step, so callers that
// retain it must clone (forEachCode itself does not clone — see the replay test,
// which clones before handing values to worker goroutines).
func forEachCode(db *pebble.DB, limit int, fn func(code []byte) error) error {
	iter, err := db.NewIter(&pebble.IterOptions{})
	if err != nil {
		return err
	}
	defer iter.Close()
	n := 0
	for iter.First(); iter.Valid(); iter.Next() {
		if limit > 0 && n >= limit {
			break
		}
		if err := fn(iter.Value()); err != nil {
			return err
		}
		n++
	}
	return iter.Error()
}

// openCorpus opens the pebble database read-only. Pebble takes no exclusive
// directory lock in read-only mode, so a replay can run against a database a
// generate campaign is still writing to.
func openCorpus(path string) (*pebble.DB, error) {
	return pebble.Open(path, &pebble.Options{ReadOnly: true, ErrorIfNotExists: true})
}
