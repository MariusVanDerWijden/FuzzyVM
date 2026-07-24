package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/MariusVanDerWijden/FuzzyVM/generator"
	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/common"
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
		// Replay twice: once with the fuzzer's own (empty-storage) pre-state, and
		// once with committed non-zero storage. The generator can only ever build
		// two accounts with empty storage, which makes the whole `original != 0`
		// half of the EIP-2200/3529 SSTORE gas-and-refund state machine
		// structurally unreachable — the single most bug-prone gas logic in the
		// EVM. Seeding storage here reaches it without changing what the fuzzer
		// stores. The first error wins; a failure in one variant shouldn't hide
		// the other.
		gst := generator.CreateGstMaker(filler.NewFiller(replaySeed), code)
		err := executeState(gst)

		warm := generator.CreateGstMaker(filler.NewFiller(replaySeed), code)
		seedPreState(warm)
		if werr := executeState(warm); err == nil {
			err = werr
		}
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(replayTimeout):
		return fmt.Errorf("replay exceeded %s", replayTimeout)
	}
}

// prestateSlots is the committed storage written to the destination account
// before replaying with a non-empty pre-state.
//
// The slots are clustered in the low range because that is where the generator's
// SSTORE/SLOAD strategies write (they draw slots from filler.MemInt, which is
// heavily biased to 0..255), so a generated program is likely to actually touch
// one of them. The values cover the cases the refund machine branches on:
// a small value, an all-ones word, and the sign bit.
var prestateSlots = map[uint64][]byte{
	0x00: {0x01},
	0x01: {0xff},
	0x02: {0x42},
	0x10: {0xde, 0xad, 0xbe, 0xef},
	0x20: {0x80, 0x00, 0x00, 0x00},
	0xff: {0x01},
}

// seedPreState gives the destination account committed, non-zero storage (and a
// non-zero balance and nonce) so a replay reaches state the generator cannot
// produce: the SSTORE original!=0 refund branches, SLOAD of a warm non-zero
// slot, a non-zero SELFBALANCE, and the "account exists with nonce" distinctions
// that EXTCODEHASH and CREATE address derivation depend on.
func seedPreState(gst *fuzzing.GstMaker) {
	dest := gst.GetDestination()
	storage := make(map[common.Hash]common.Hash, len(prestateSlots))
	for slot, val := range prestateSlots {
		storage[common.BigToHash(new(big.Int).SetUint64(slot))] = common.BytesToHash(val)
	}
	// AddAccount overwrites by address, so this replaces the destination the
	// generator added — keeping its code, which is the program under replay.
	gst.AddAccount(dest, fuzzing.GenesisAccount{
		Code:    codeOf(gst, dest),
		Storage: storage,
		Balance: big.NewInt(0x1bc16d674ec80000), // 2 ETH, so SELFBALANCE is non-zero
		Nonce:   7,
	})
}

// codeOf digs the code currently assigned to addr out of the state test, so
// seedPreState can rewrite the account without losing the program under replay.
func codeOf(gst *fuzzing.GstMaker, addr common.Address) []byte {
	name := ""
	gstPtr := gst.ToGeneralStateTest(name)
	if sub, ok := (*gstPtr)[name]; ok {
		if acc, ok := sub.Pre[addr]; ok {
			return acc.Code
		}
	}
	return nil
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
