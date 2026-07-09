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
	"fmt"
	"math"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
)

type Environment struct {
	f         *filler.Filler
	p         *program.Program
	jumptable *Jumptable
	// recursionLevel is how deeply nested we are in createCallGenerator (which
	// recursively generates sub-programs). It is per-generation, carried down
	// through the nested GenerateProgram calls, so it resets for every
	// top-level generation instead of leaking across a fuzz worker's lifetime.
	recursionLevel int
	// labels caches the PCs of real JUMPDESTs emitted so far in this program.
	// Structured control-flow strategies (jump_strategies) jump to these known
	// destinations, so JUMP/JUMPI always target a valid JUMPDEST instead of the
	// jumptable's placeholder-scanning heuristic. It is a pointer so the slice
	// survives Environment being passed by value.
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

// Probability returns the probability of this strategy,
// given the sum of all strategies on scale 1-255.
func Probability(strat Strategy) byte {
	imp := strat.Importance()
	return max(byte(math.Round((float64(imp)/float64(100))*float64(255))), 1)
}

// makeMap builds the 256-entry strategy selection table. Each strategy claims a
// number of slots proportional to its Probability; any slots left over are
// filled with validOpcodeGenerator so every possible selection byte maps to a
// strategy (generator.go relies on this: a nil entry panics).
func makeMap(strats []Strategy) map[byte]Strategy {
	m := make(map[byte]Strategy)
	sum := 0
	for _, strat := range strats {
		prob := int(Probability(strat))
		if sum+prob > 256 {
			// The weighted slots would overflow the 256-entry table and start
			// overwriting earlier strategies. Refuse loudly instead of silently
			// corrupting selection — lower some Importance values or switch to a
			// wider selection table.
			panic(fmt.Sprintf("strategy probabilities exceed 256 (at %q, running total %d); reduce Importance values", strat.String(), sum+prob))
		}
		for i := 0; i < prob; i++ {
			m[byte(sum)] = strat
			sum++
		}
	}
	// Fill any remaining slots [sum, 256) with the fallback generator.
	for i := sum; i < 256; i++ {
		m[byte(i)] = new(validOpcodeGenerator)
	}
	return m
}

func (env Environment) CreateAndCall(code []byte, isCreate2 bool, callOp vm.OpCode) {
	var (
		value    = 0
		offset   = 0
		size     = len(code)
		salt     = 0
		createOp = vm.CREATE
	)
	// Load the code into mem
	env.p.Mstore(code, 0)
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
