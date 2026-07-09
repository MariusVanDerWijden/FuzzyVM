package generator

import (
	"bytes"
	"strings"
	"testing"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/core/vm/program"
)

// TestBoundedLoopExecutes runs a program made only of bounded loops through the
// state test filler and confirms it terminates well under the 100M gas limit,
// i.e. the loop is genuinely bounded rather than running until out of gas.
func TestBoundedLoopExecutes(t *testing.T) {
	labels := []uint64{}
	env := Environment{
		p:         program.New(),
		f:         filler.NewFiller([]byte{5, 7, 3, 9, 200, 1, 2, 3, 4, 8}),
		jumptable: NewJumptable(10),
		labels:    &labels,
	}
	var g boundedLoopGenerator
	for i := 0; i < 3; i++ {
		g.Execute(env)
	}
	code := env.p.Bytes()

	f := filler.NewFiller([]byte("seed-for-gstmaker-padding-bytes-to-avoid-wrap"))
	gst := CreateGstMaker(f, code)
	var trace bytes.Buffer
	if err := gst.Fill(&trace); err != nil {
		t.Fatalf("Fill failed: %v", err)
	}
	// A runaway loop produces an enormous trace; three bounded loops of <=16
	// iterations produce a tiny one. Guard generously.
	if n := strings.Count(trace.String(), "\n"); n > 5000 {
		t.Fatalf("trace has %d lines; loop may not be bounded", n)
	}
}
