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

var stackStrategies = []Strategy{
	new(stackAwareGenerator),
	new(arithEdgeGenerator),
}

// Interesting 256-bit operand values. These are the boundary values where EVM
// arithmetic, comparison and bit operations historically diverge across
// clients: zero, small ints, the all-ones word (-1 / 2^256-1), the sign bit
// (2^255 = MIN_INT in two's complement), and word/bit boundaries.
var (
	maxUint256 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)) // 2^256-1
	minInt256  = new(big.Int).Lsh(big.NewInt(1), 255)                                   // 2^255
)

var interestingOperands = []*big.Int{
	big.NewInt(0),
	big.NewInt(1),
	big.NewInt(2),
	big.NewInt(31),
	big.NewInt(32),
	big.NewInt(255),
	big.NewInt(256),
	maxUint256,
	minInt256,
	new(big.Int).Sub(minInt256, big.NewInt(1)), // 2^255-1 = MAX_INT
}

// interestingOperand returns an operand to push. Most of the time it is drawn
// from the boundary set above; occasionally it is a fully random word so the
// generator still explores the middle of the value space.
func interestingOperand(env Environment) *big.Int {
	if env.f.Byte() < 64 {
		// ~1/4: a random 256-bit value.
		return env.f.BigInt256()
	}
	choices := interestingOperands
	return choices[int(env.f.Byte())%len(choices)]
}

// pushOperand pushes one operand and records it on the modeled stack.
func (env Environment) pushOperand() {
	env.p.Push(interestingOperand(env))
	*env.stackHeight++
}

// ensureStack pushes operands until the modeled stack holds at least n items.
func (env Environment) ensureStack(n int) {
	for *env.stackHeight < n {
		env.pushOperand()
	}
}

// applyStackDelta updates the modeled stack height for an op that pops `pop`
// items and pushes `push`, clamping at zero so the model can't go negative.
func (env Environment) applyStackDelta(pop, push int) {
	h := *env.stackHeight - pop + push
	if h < 0 {
		h = 0
	}
	*env.stackHeight = h
}

// stackOp describes an opcode's effect on the stack.
type stackOp struct {
	op   vm.OpCode
	pop  int
	push int
}

// stackAwareOps are opcodes worth emitting with their operands present, so they
// actually execute instead of reverting on stack underflow. Deliberately
// limited to pure, terminating, operand-driven ops: arithmetic, comparison,
// bitwise, and a few unary ops. Control flow, calls, storage and memory are
// covered by their own strategies.
var stackAwareOps = []stackOp{
	// Binary arithmetic / comparison / bitwise: pop 2, push 1.
	{vm.ADD, 2, 1}, {vm.MUL, 2, 1}, {vm.SUB, 2, 1}, {vm.DIV, 2, 1},
	{vm.SDIV, 2, 1}, {vm.MOD, 2, 1}, {vm.SMOD, 2, 1}, {vm.EXP, 2, 1},
	{vm.SIGNEXTEND, 2, 1}, {vm.LT, 2, 1}, {vm.GT, 2, 1}, {vm.SLT, 2, 1},
	{vm.SGT, 2, 1}, {vm.EQ, 2, 1}, {vm.AND, 2, 1}, {vm.OR, 2, 1},
	{vm.XOR, 2, 1}, {vm.BYTE, 2, 1}, {vm.SHL, 2, 1}, {vm.SHR, 2, 1},
	{vm.SAR, 2, 1},
	// Ternary: pop 3, push 1.
	{vm.ADDMOD, 3, 1}, {vm.MULMOD, 3, 1},
	// Unary: pop 1, push 1.
	{vm.ISZERO, 1, 1}, {vm.NOT, 1, 1},
}

// stackAwareGenerator picks an operand-driven opcode and pushes interesting
// operands until its inputs are present, then emits it. This is the core of the
// stack model: without it, a bare opcode almost always hits an empty stack and
// the program reverts on underflow before reaching any interesting behaviour.
type stackAwareGenerator struct{}

func (*stackAwareGenerator) Execute(env Environment) {
	// The modeled height is only maintained by the stack-aware strategies; any
	// other strategy that ran in between may have popped the real stack without
	// updating the model, leaving it overcounting relative to reality. An
	// overcount is the dangerous direction: ensureStack would then trust items
	// that aren't there and emit an op that underflows and reverts — the exact
	// case this model exists to avoid. So distrust a possibly-stale height and
	// re-establish the operands from scratch. The cost is a few redundant pushes
	// when the model happened to be accurate; the benefit is that the op never
	// underflows regardless of what ran before it.
	*env.stackHeight = 0
	so := stackAwareOps[int(env.f.Byte())%len(stackAwareOps)]
	env.ensureStack(so.pop)
	env.p.Op(so.op)
	env.applyStackDelta(so.pop, so.push)
}

func (*stackAwareGenerator) Importance() int {
	return 12
}

func (*stackAwareGenerator) String() string {
	return "stackAwareGenerator"
}

// arithEdgeGenerator emits fully-specified arithmetic edge cases that are
// classic cross-client divergence points. Each case pushes its exact operands
// (so it doesn't depend on the modeled stack) and leaves one result on the
// stack, which it accounts for in the model.
type arithEdgeGenerator struct{}

func (*arithEdgeGenerator) Execute(env Environment) {
	// Operands are pushed in reverse order: the last Push is the top of stack,
	// i.e. the first operand the opcode pops.
	switch env.f.Byte() % 9 {
	case 0:
		// ADDMOD with modulus 0 (defined to return 0).
		env.p.Push(big.NewInt(0)).Push(maxUint256).Push(maxUint256).Op(vm.ADDMOD)
	case 1:
		// MULMOD with modulus 0 (defined to return 0).
		env.p.Push(big.NewInt(0)).Push(maxUint256).Push(maxUint256).Op(vm.MULMOD)
	case 2:
		// SDIV of MIN_INT by -1: overflows the signed range, defined to wrap.
		env.p.Push(maxUint256).Push(minInt256).Op(vm.SDIV) // -1 == 2^256-1
	case 3:
		// SMOD of MIN_INT by -1 (result 0).
		env.p.Push(maxUint256).Push(minInt256).Op(vm.SMOD)
	case 4:
		// EXP(0, 0), defined to be 1.
		env.p.Push(big.NewInt(0)).Push(big.NewInt(0)).Op(vm.EXP)
	case 5:
		// EXP with a large exponent (gas + big-int stress).
		env.p.Push(maxUint256).Push(big.NewInt(2)).Op(vm.EXP)
	case 6:
		// Shifts at the 255/256 bit boundary.
		shift := big.NewInt(255)
		if env.f.Bool() {
			shift = big.NewInt(256)
		}
		op := vm.SHL
		switch env.f.Byte() % 3 {
		case 1:
			op = vm.SHR
		case 2:
			op = vm.SAR
		}
		env.p.Push(minInt256).Push(shift).Op(op)
	case 7:
		// SIGNEXTEND at a byte boundary of a negative-looking value.
		env.p.Push(maxUint256).Push(big.NewInt(0)).Op(vm.SIGNEXTEND)
	case 8:
		// BYTE indexing at and past the 32-byte boundary.
		idx := big.NewInt(31)
		if env.f.Bool() {
			idx = big.NewInt(32) // out of range, defined to return 0
		}
		env.p.Push(maxUint256).Push(idx).Op(vm.BYTE)
	}
	// Every case above pushes its operands and emits one op that leaves a single
	// result. Net stack effect: +1.
	*env.stackHeight++
}

func (*arithEdgeGenerator) Importance() int {
	return 6
}

func (*arithEdgeGenerator) String() string {
	return "arithEdgeGenerator"
}
