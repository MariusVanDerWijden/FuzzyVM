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
	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
)

type Environment struct {
	f         *filler.Filler
	p         *program.Program
	jumptable *Jumptable
}

type Strategy interface {
	// Execute executes the strategy.
	// adds the resulting opcodes to the program.
	Execute(env Environment)
	// Importance returns the importance of this strategy.
	// This is needed to calculate the probability of this strategy.
	// Should be on a scale of 1-100.
	Importance() int
}

// Probability returns the probability of this strategy,
// given the sum of all strategies on scale 1-255.
func Probability(strat Strategy, sum int) byte {
	imp := strat.Importance()
	pr := byte(float32(sum) / float32(imp) * 255)
	if pr == 0 {
		return 1
	}
	return pr
}

// accStrat is an accumulated strategy.
// It has a range between 0-255 in which this strategy is executed.
type accStrat struct {
	rnge  byte
	strat Strategy
}

func newAccStrats(strats []Strategy) []accStrat {
	sum := 0
	for _, s := range strats {
		sum += s.Importance()
	}
	rnge := byte(0)
	res := make([]accStrat, len(strats))
	for i, s := range strats {
		res[i] = accStrat{
			rnge:  rnge,
			strat: s,
		}
		rnge += Probability(s, sum)
	}
	return res
}

// selectStrat selects the strategy that has the range
// specified by the rnd byte.
// expects acc to be ordered.
func selectStrat(rnd byte, acc []accStrat) Strategy {
	for i := 0; i < len(acc)-1; i++ {
		if rnd > acc[i].rnge && rnd < acc[i+1].rnge {
			return acc[i].strat
		}
	}
	return acc[len(acc)-1].strat
}

func (env Environment) CreateAndCall(code []byte, isCreate2 bool, callOp vm.OpCode) {
	if isCreate2 {
		env.p.Create2ThenCall(code, []byte{123})
	} else {
		var (
			value  = 0
			offset = 0
			size   = len(code)
		)
		// Load the code into mem
		env.p.Mstore(code, 0)
		// Create it
		env.p.Push(size)
		env.p.Push(offset)
		env.p.Push(value)
		env.p.Op(vm.CREATE)
		// If there happen to be a zero on the stack, it doesn't matter, we're
		// not sending any value anyway
		env.p.Push(0).Push(0) // mem out
		env.p.Push(0).Push(0) // mem in
		if callOp == vm.CALL || callOp == vm.CALLCODE {
			env.p.Push(0)     // value
			env.p.Op(vm.DUP6) // address
		} else {
			env.p.Op(vm.DUP5) // address
		}
		env.p.Op(vm.GAS)
		env.p.Op(callOp) // opcode
		env.p.Op(vm.POP) // pop the retval
		env.p.Op(vm.POP) // pop the address
	}
}
