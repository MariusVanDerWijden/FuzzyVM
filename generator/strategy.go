// Copyright 2021 Marius van der Wijden
// This file is part of the fuzzy-vm library.
//
// The fuzzy-vm library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The fuzzy-vm library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the fuzzy-vm library. If not, see <http://www.gnu.org/licenses/>.

package generator

import (
	"sort"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
)

// Environment is passed to strategies by value. Note the deliberate split:
// pointer fields (p, labels, stackHeight) let a strategy mutate shared state
// that outlives the call, whereas value fields (recursionLevel) are per-call
// snapshots. A strategy that tried to persist a change to a value field
// (e.g. env.recursionLevel++) would silently not propagate — recursion depth is
// instead threaded explicitly through generateCode(f, level+1).
type Environment struct {
	f *filler.Filler
	p *program.Program
	// recursionLevel is how deeply nested we are in createCallGenerator (which
	// recursively generates sub-programs). It is per-generation, carried down
	// through the nested GenerateProgram calls, so it resets for every
	// top-level generation instead of leaking across a fuzz worker's lifetime.
	recursionLevel int
	// labels caches the PCs of real JUMPDESTs emitted so far in this program.
	// The control-flow strategies (jump_strategies) jump only to these known
	// destinations, so JUMP/JUMPI always target a valid JUMPDEST. It is a pointer
	// so the slice survives Environment being passed by value.
	labels *[]uint64
	// stackHeight is a conservative model of the number of items currently on
	// the EVM stack. It is a pointer so it survives Environment being passed by
	// value. Only the stack-aware strategies maintain it: they add to it for
	// every operand they push and subtract an op's net stack effect. Other
	// strategies' stack effects are not tracked, so the model can undercount but
	// never (from its own actions) overcount — which is the safe direction,
	// since undercounting just makes the stack-aware generator push a few
	// redundant operands rather than emit an op that underflows.
	stackHeight *int
}

// addLabel emits a JUMPDEST and records its PC as a reusable jump target.
func (env Environment) addLabel() uint64 {
	_, pc := env.p.Jumpdest()
	*env.labels = append(*env.labels, pc)
	return pc
}

// randomLabel returns a cached JUMPDEST PC and true, or false if none exist.
func (env Environment) randomLabel() (uint64, bool) {
	if len(*env.labels) == 0 {
		return 0, false
	}
	return (*env.labels)[int(env.f.Byte())%len(*env.labels)], true
}

type Strategy interface {
	// Execute executes the strategy.
	// adds the resulting opcodes to the program.
	Execute(env Environment)
	// Importance returns the importance of this strategy.
	// This is needed to calculate the probability of this strategy.
	// Should be on a scale of 1-100.
	Importance() int

	String() string
}

// fallbackWeight is the weight given to the validOpcodeGenerator fallback. The
// old 256-slot table filled whatever slots the weighted strategies left over
// (~17/256) with this generator; we keep an equivalent share explicitly so raw
// random-opcode emission stays part of the mix without depending on the table
// size.
const fallbackWeight = 17

// selector chooses a strategy with probability proportional to its Importance.
// It replaces the old fixed 256-entry map[byte]Strategy, which capped the total
// weight at 256 and panicked once the strategies' importances summed past it.
// Instead it keeps a cumulative-weight table and binary-searches a value drawn
// from the filler, so any number of strategies (and any importances) fit.
type selector struct {
	strats []Strategy
	// cum[i] is the running sum of weights up to and including strats[i]; the
	// last element is the total weight.
	cum   []int
	total int
}

// newSelector builds the weighted selector. Weights are the raw Importance
// values; a validOpcodeGenerator fallback is appended so every selection lands
// on a real strategy (generator relies on Select never returning nil).
func newSelector(strats []Strategy) *selector {
	all := make([]Strategy, 0, len(strats)+1)
	all = append(all, strats...)
	all = append(all, new(validOpcodeGenerator))

	s := &selector{strats: all}
	s.cum = make([]int, len(all))
	for i, strat := range all {
		w := strat.Importance()
		if _, ok := strat.(*validOpcodeGenerator); ok {
			w = fallbackWeight
		}
		if w < 1 {
			w = 1
		}
		s.total += w
		s.cum[i] = s.total
	}
	if s.total <= 0 {
		panic("selector total weight must be positive")
	}
	return s
}

// Select draws a strategy from the filler. It consumes two bytes so the weighted
// space can be larger than 256 (the old byte-indexed table could not).
func (s *selector) Select(f *filler.Filler) Strategy {
	r := int(f.Uint16()) % s.total
	// Binary search for the first cumulative weight strictly greater than r.
	i := sort.Search(len(s.cum), func(i int) bool { return s.cum[i] > r })
	return s.strats[i]
}

// CreateAndCall runs initCode as constructor code (CREATE/CREATE2) and then
// calls the resulting account with callOp.
func (env Environment) CreateAndCall(initCode []byte, isCreate2 bool, callOp vm.OpCode) {
	var (
		value    = 0
		offset   = 0
		size     = len(initCode)
		salt     = 0
		createOp = vm.CREATE
	)
	// Load the code into mem
	env.p.Mstore(initCode, 0)
	// Create it
	if isCreate2 {
		env.p.Push(salt)
		createOp = vm.CREATE2
	}
	env.p.Push(size).Push(offset).Push(value).Op(createOp)
	// If there happen to be a zero on the stack, it doesn't matter, we're
	// not sending any value anyway
	env.p.Push(0).Push(0) // mem out
	env.p.Push(0).Push(0) // mem in
	addrOffset := vm.OpCode(vm.DUP5)
	if callOp == vm.CALL || callOp == vm.CALLCODE {
		env.p.Push(0) // value
		addrOffset = vm.DUP6
	}
	env.p.Op(addrOffset) // address (from create-op above)
	env.p.Op(vm.GAS, callOp)
}
