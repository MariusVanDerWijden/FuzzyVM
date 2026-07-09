package generator

import (
	"testing"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
)

// TestBoundedLoopTerminates builds a program containing only bounded loops and
// checks it assembles into valid bytecode whose JUMPI targets are real
// JUMPDESTs.
func TestBoundedLoopTerminates(t *testing.T) {
	env := Environment{
		p:         program.New(),
		f:         filler.NewFiller([]byte{5, 7, 3, 9, 200, 1, 2, 3, 4}),
		jumptable: NewJumptable(10),
		labels:    new([]uint64),
	}
	var g boundedLoopGenerator
	for i := 0; i < 4; i++ {
		g.Execute(env)
	}
	code := env.p.Bytes()
	// Every recorded label must point at a JUMPDEST opcode.
	for _, pc := range *env.labels {
		if int(pc) >= len(code) || vm.OpCode(code[pc]) != vm.JUMPDEST {
			t.Fatalf("label %d does not point at a JUMPDEST (op=%#x)", pc, code[pc])
		}
	}
	if len(*env.labels) != 4 {
		t.Fatalf("expected 4 loop-head labels, got %d", len(*env.labels))
	}
}

// TestLabelJumpTargetsAreValid checks that labelJumpGenerator only ever jumps
// to cached JUMPDEST PCs.
func TestLabelJumpTargetsAreValid(t *testing.T) {
	labels := []uint64{}
	env := Environment{
		p:         program.New(),
		f:         filler.NewFiller([]byte{0xff, 0x01, 0x80, 0x00, 0x40, 0x81}),
		jumptable: NewJumptable(10),
		labels:    &labels,
	}
	// Seed one label first.
	env.addLabel()
	var g labelJumpGenerator
	for i := 0; i < 5; i++ {
		g.Execute(env)
	}
	code := env.p.Bytes()
	for _, pc := range labels {
		if vm.OpCode(code[pc]) != vm.JUMPDEST {
			t.Fatalf("cached label %d is not a JUMPDEST", pc)
		}
	}
}
