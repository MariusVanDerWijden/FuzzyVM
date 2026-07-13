package generator

import (
	"strings"
	"testing"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/core/vm/program"
)

func newStackEnv(seed []byte) (Environment, *[]uint64, *int) {
	labels := []uint64{}
	h := 0
	return Environment{
		p:           program.New(),
		f:           filler.NewFiller(seed),
		labels:      &labels,
		stackHeight: &h,
	}, &labels, &h
}

// TestStackAwarePushesOperands checks that stackAwareGenerator never emits an op
// without first ensuring its operands are on the modeled stack.
func TestStackAwarePushesOperands(t *testing.T) {
	env, _, h := newStackEnv([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})
	var g stackAwareGenerator
	for i := 0; i < 20; i++ {
		before := *h
		g.Execute(env)
		// After each op the modeled height must be non-negative and consistent:
		// the op only ran if enough operands were present.
		if *h < 0 {
			t.Fatalf("modeled stack height went negative: %d", *h)
		}
		_ = before
	}
	if len(env.p.Bytes()) == 0 {
		t.Fatal("no code generated")
	}
}

// TestArithEdgeExecutes builds a program of only arithmetic edge cases and runs
// it through the filler/EVM; it must fill without error and produce a
// non-trivial trace (i.e. it actually executed, not an instant revert).
func TestArithEdgeExecutes(t *testing.T) {
	env, _, _ := newStackEnv([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 42, 43, 44, 45, 46})
	var g arithEdgeGenerator
	for i := 0; i < 9; i++ {
		g.Execute(env)
	}
	code := env.p.Bytes()

	f := filler.NewFiller([]byte("padding-seed-for-gstmaker-transaction-fields"))
	gst := CreateGstMaker(f, code)
	var trace traceCounter
	if err := gst.Fill(&trace, 0); err != nil {
		t.Fatalf("Fill failed: %v", err)
	}
	// Each edge case is several ops; a program of 9 that instantly reverted
	// would trace only a handful of steps. Require meaningfully more.
	if trace.lines < 20 {
		t.Fatalf("trace only %d lines; program likely reverted early", trace.lines)
	}
}

type traceCounter struct{ lines int }

func (t *traceCounter) Write(p []byte) (int, error) {
	t.lines += strings.Count(string(p), "\n")
	return len(p), nil
}
