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
	"math/big"

	"github.com/ethereum/go-ethereum/core/vm"
)

var jumpStrategies = []Strategy{
	new(jumpdestGenerator),
	new(jumpGenerator),
	new(labelJumpGenerator),
	new(boundedLoopGenerator),
}

type jumpdestGenerator struct{}

func (*jumpdestGenerator) Execute(env Environment) {
	// Emit a real JUMPDEST and cache its PC as a reusable jump target for the
	// label-based control-flow strategies.
	env.addLabel()
}

func (*jumpdestGenerator) Importance() int {
	return 5
}

func (*jumpdestGenerator) String() string {
	return "jumpdestGenerator"
}

// jumpGenerator emits a JUMP or JUMPI to one of the real JUMPDESTs emitted so
// far. Like labelJumpGenerator it targets a cached, valid label so the jump
// actually lands; unlike it, the JUMPI condition is drawn from the filler
// (BigInt32) rather than a fixed 0/1, so it exercises a wider set of
// condition values. This replaces the old jumptable placeholder mechanism,
// which wrote a 0xFF...FF sentinel and post-scanned the bytecode to patch it,
// producing mostly-invalid jumps.
type jumpGenerator struct{}

func (*jumpGenerator) Execute(env Environment) {
	dest, ok := env.randomLabel()
	if !ok {
		// No labels yet; emit one so later jumps have somewhere to go.
		env.addLabel()
		return
	}
	if env.f.Bool() {
		// Unconditional jump to a valid destination.
		env.p.Jump(dest)
	} else {
		// Conditional jump: with a fuzzed condition (zero => not taken).
		condition := big.NewInt(0)
		if env.f.Bool() {
			condition = env.f.BigInt32()
		}
		env.p.JumpIf(dest, condition)
	}
}

func (*jumpGenerator) Importance() int {
	return 7
}

func (*jumpGenerator) String() string {
	return "jumpGenerator"
}

// labelJumpGenerator jumps to one of the real JUMPDESTs emitted so far. Because
// the destination is a cached, valid label, the jump actually lands (unlike the
// jumptable heuristic, which frequently poisons the jump). A JUMP here is an
// unconditional (usually backward) branch; a JUMPI is taken only some of the
// time, exercising both edges of the branch.
type labelJumpGenerator struct{}

func (*labelJumpGenerator) Execute(env Environment) {
	dest, ok := env.randomLabel()
	if !ok {
		// No labels yet; emit one so later jumps have somewhere to go.
		env.addLabel()
		return
	}
	if env.f.Bool() {
		// Conditional branch: taken iff the condition is non-zero. Flip a coin
		// so the corpus contains both taken and not-taken executions.
		condition := big.NewInt(0)
		if env.f.Bool() {
			condition = big.NewInt(1)
		}
		env.p.JumpIf(dest, condition)
	} else {
		env.p.Jump(dest)
	}
}

func (*labelJumpGenerator) Importance() int {
	return 5
}

func (*labelJumpGenerator) String() string {
	return "labelJumpGenerator"
}

// boundedLoopGenerator emits a counter-driven loop that is guaranteed to
// terminate: it initialises a counter, and each iteration decrements it and
// branches back to the loop head while it is non-zero. This stresses gas
// metering and per-iteration state without the risk of an unbounded
// (until-out-of-gas) loop that the raw jumptable can produce.
type boundedLoopGenerator struct{}

func (*boundedLoopGenerator) Execute(env Environment) {
	// Keep the iteration count small so a loop nested inside other loops can't
	// blow up execution time. 1..16 iterations.
	iterations := int64(env.f.Byte()%16) + 1
	// Push initial counter value.
	env.p.Push(big.NewInt(iterations))
	// Loop head.
	head := env.addLabel()
	// Body: a couple of cheap, side-effecting ops so the loop isn't empty.
	env.p.Op(vm.GAS, vm.POP)
	// counter = counter - 1 (counter is on top of stack).
	env.p.Push(big.NewInt(1))
	env.p.Op(vm.SWAP1, vm.SUB)
	// Branch back to head while the counter is non-zero. DUP1 copies the
	// counter to use as the JUMPI condition (JUMPI pops [dest, condition] with
	// dest on top), leaving the original counter for the next iteration. We
	// can't use program.JumpIf here: it pushes its own condition, whereas we
	// need the condition to come from the stack.
	env.p.Op(vm.DUP1)
	env.p.Push(head)
	env.p.Op(vm.JUMPI)
	// Clean up the leftover zero counter.
	env.p.Op(vm.POP)
}

func (*boundedLoopGenerator) Importance() int {
	return 4
}

func (*boundedLoopGenerator) String() string {
	return "boundedLoopGenerator"
}
