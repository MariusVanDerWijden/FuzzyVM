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

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/holiman/uint256"
)

// coverageStrategies target specific, branch-dense core/vm paths that the
// generic strategies rarely reach: write-protection, warm/cold access, the
// SSTORE refund machine, structured CALL patterns, RETURNDATACOPY/MCOPY bounds,
// LOG gas, EIP-6780 self-destruct, invalid-jump analysis, and the stack limit.
// Each builds its own operands, so it does not depend on the modeled stack;
// importances are kept low (1-3) so they enrich rather than dominate the mix.
var coverageStrategies = []Strategy{
	new(staticContextGenerator),
	new(warmColdGenerator),
	new(sstoreWalkGenerator),
	new(valueCallGenerator),
	new(deepCallGenerator),
	new(returnDataCopyGenerator),
	new(mcopyGenerator),
	new(logGenerator),
	new(selfdestructGenerator),
	new(invalidJumpGenerator),
	new(stackFillGenerator),
}

// P1. Run a generated child under STATICCALL — write-protection paths.
// Exercises the readOnly -> ErrWriteProtection branches of SSTORE/LOG*/value-
// CALL/CREATE/SELFDESTRUCT and the read-only gas-table guards at once.
type staticContextGenerator struct{}

func (*staticContextGenerator) Execute(env Environment) {
	if env.recursionLevel >= maxRecursionLevel {
		return
	}
	// Deploy a real child contract whose runtime *begins* with a state-writing
	// op, then STATICCALL it.
	child := append(writeOp(env.f), generateCode(filler.NewFiller(env.f.ByteSlice(int(env.f.Uint16()))), env.recursionLevel+1)...)
	env.CreateAndCall(deployInitCode(child), false, vm.STATICCALL)
}

func (*staticContextGenerator) Importance() int { return 3 }
func (*staticContextGenerator) String() string  { return "staticContextGenerator" }

// P2. Warm/cold access pairs (EIP-2929). Touching the same slot/address twice
// takes the cold path then the warm path — the warm branch is otherwise rarely
// hit by a single random touch.
type warmColdGenerator struct{}

func (*warmColdGenerator) Execute(env Environment) {
	op := []vm.OpCode{vm.SLOAD, vm.BALANCE, vm.EXTCODESIZE, vm.EXTCODEHASH}[env.f.Byte()%4]
	key := env.f.BigInt256() // storage slot, or address in the low 20 bytes
	for i := 0; i < 2; i++ {
		env.p.Push(key).Op(op).Op(vm.POP)
	}
}

func (*warmColdGenerator) Importance() int { return 2 }
func (*warmColdGenerator) String() string  { return "warmColdGenerator" }

// P3. SSTORE refund state machine. Walking one slot through a value sequence
// traverses create -> dirty-update -> delete-refund -> reset in a single
// program. The original != 0 cases need committed pre-state (differential
// pipeline); the original == 0 half is reachable from the corpus.
type sstoreWalkGenerator struct{}

func (*sstoreWalkGenerator) Execute(env Environment) {
	slot := uint32(env.f.Uint16())
	// 0 -> A (create), A -> B (dirty update), B -> 0 (delete, +refund),
	// 0 -> C (recreate), C -> 0 (delete, +refund). An empty slice == value 0.
	for _, v := range [][]byte{{0x01}, {0x02}, {}, {0x03}, {}} {
		env.p.Sstore(slot, v)
	}
}

func (*sstoreWalkGenerator) Importance() int { return 2 }
func (*sstoreWalkGenerator) String() string  { return "sstoreWalkGenerator" }

// P4a. Value-bearing CALL, twice to the same address: exercises the 2300 gas
// stipend, the new-account charge (if the target is empty), and cold-then-warm
// access.
type valueCallGenerator struct{}

func (*valueCallGenerator) Execute(env Environment) {
	gas := uint256.NewInt(uint64(env.f.Uint16()) + 2300)
	addr := common.BytesToAddress(env.f.ByteSlice(20))
	value := big.NewInt(int64(env.f.Uint16()))
	for i := 0; i < 2; i++ {
		env.p.Call(gas, addr, value, 0, 0, 0, 0).Op(vm.POP)
	}
}

func (*valueCallGenerator) Importance() int { return 2 }
func (*valueCallGenerator) String() string  { return "valueCallGenerator" }

// P4b. Self-call with all remaining gas: the child re-runs this whole program
// and self-calls again, recursing toward the 1024 depth limit (ErrDepth).
type deepCallGenerator struct{}

func (*deepCallGenerator) Execute(env Environment) {
	// CALL pops gas, addr, value, argsOff, argsSize, retOff, retSize (top-first),
	// so push the tail first and supply addr and gas via ADDRESS then GAS.
	env.p.Push(0).Push(0).Push(0).Push(0).Push(0) // retSize,retOff,argsSize,argsOff,value
	env.p.Op(vm.ADDRESS, vm.GAS, vm.CALL, vm.POP)
}

func (*deepCallGenerator) Importance() int { return 1 }
func (*deepCallGenerator) String() string  { return "deepCallGenerator" }

// P5. RETURNDATACOPY lifecycle + out-of-bounds. Call a child that returns a
// known size, then copy within and past it (offset+length > len(returnData)
// reverts with ErrReturnDataOutOfBounds).
type returnDataCopyGenerator struct{}

func (*returnDataCopyGenerator) Execute(env Environment) {
	// Deploy a child whose *runtime* returns 32 bytes, then STATICCALL it so the
	// 32 bytes land in the returndata buffer.
	childRuntime := program.New().Return(0, 32).Bytes()
	env.CreateAndCall(deployInitCode(childRuntime), false, vm.STATICCALL) // sets 32-byte returnData
	// RETURNDATACOPY pops destOffset, dataOffset, length (top-first). A length
	// > 32 overruns the buffer -> ErrReturnDataOutOfBounds.
	length := int(env.f.Byte()) // 0..255, sometimes > 32
	env.p.Push(length).Push(0).Push(0).Op(vm.RETURNDATACOPY)
}

func (*returnDataCopyGenerator) Importance() int { return 2 }
func (*returnDataCopyGenerator) String() string  { return "returnDataCopyGenerator" }

// P6. MCOPY overlap (EIP-5656). Overlapping src/dst is a specific divergence
// point reachable today only via a bare random opcode (which underflows).
type mcopyGenerator struct{}

func (*mcopyGenerator) Execute(env Environment) {
	env.p.Mstore(env.f.ByteSlice256(), 0) // seed memory
	// Bias toward overlapping ranges within the seeded region.
	dst := int(env.f.Byte()) % 64
	src := int(env.f.Byte()) % 64
	length := int(env.f.Byte()) // includes 0 and memory-expanding sizes
	// MCOPY pops destOffset, srcOffset, length (top-first).
	env.p.Push(length).Push(src).Push(dst).Op(vm.MCOPY)
}

func (*mcopyGenerator) Importance() int { return 2 }
func (*mcopyGenerator) String() string  { return "mcopyGenerator" }

// P7. LOG0-LOG4 with topics + data. Gas scales with topic count and data size,
// and makeLog has the read-only guard. Not generated with real operands today.
type logGenerator struct{}

func (*logGenerator) Execute(env Environment) {
	env.p.Mstore(env.f.ByteSlice256(), 0)
	n := int(env.f.Byte() % 5) // LOG0..LOG4
	// Stack (top->bottom): mStart, mSize, topic0, topic1, ... So push topics
	// deepest-first, then mSize, then mStart.
	for i := n - 1; i >= 0; i-- {
		env.p.Push(env.f.BigInt256()) // topic
	}
	env.p.Push(int(env.f.Byte())) // mSize
	env.p.Push(0)                 // mStart
	env.p.Op(vm.LOG0 + vm.OpCode(n))
}

func (*logGenerator) Importance() int { return 2 }
func (*logGenerator) String() string  { return "logGenerator" }

// P8. SELFDESTRUCT / EIP-6780 same-tx deletion. A child created and called in
// the same tx that self-destructs hits the "created this transaction" branch
// that actually deletes the account (a top-level SELFDESTRUCT only transfers).
type selfdestructGenerator struct{}

func (*selfdestructGenerator) Execute(env Environment) {
	child := program.New()
	child.Push(env.f.BigInt256()) // beneficiary (address in low 20 bytes)
	child.Op(vm.SELFDESTRUCT)
	env.CreateAndCall(child.Bytes(), false, vm.CALL)
}

func (*selfdestructGenerator) Importance() int { return 1 }
func (*selfdestructGenerator) String() string  { return "selfdestructGenerator" }

// P9. Invalid jump into PUSH data (jumpdest analysis). A 0x5b byte inside PUSH
// immediate data is not a valid JUMPDEST; a jump to it must be rejected with
// ErrInvalidJump. The label strategies only jump to real JUMPDESTs, so this
// path is otherwise untested.
type invalidJumpGenerator struct{}

func (*invalidJumpGenerator) Execute(env Environment) {
	at := env.p.Label()      // PC of the PUSH1 we are about to emit
	env.p.Push([]byte{0x5b}) // PUSH1 0x5b — the 0x5b sits at PC at+1 (inside pushdata)
	env.p.Op(vm.POP)
	env.p.Jump(at + 1) // jump into the pushdata byte -> ErrInvalidJump
}

func (*invalidJumpGenerator) Importance() int { return 1 }
func (*invalidJumpGenerator) String() string  { return "invalidJumpGenerator" }

// P10. Stack-limit boundary. Approach the 1024-item stack limit so a DUP trips
// stack overflow. It intentionally leaves the stack deep, so it resets the
// modeled height to keep later stack-aware ops honest — and is best emitted
// late (its high per-op count and low importance make that likely).
type stackFillGenerator struct{}

func (*stackFillGenerator) Execute(env Environment) {
	env.p.Push0()
	for i := 0; i < 1000+int(env.f.Byte())%40; i++ { // approach/exceed 1024
		env.p.Op(vm.DUP1)
	}
	// The real stack is now very deep; the model can't track that usefully, so
	// clamp it back to zero rather than leave a huge count that would make
	// ensureStack a no-op forever.
	*env.stackHeight = 0
}

func (*stackFillGenerator) Importance() int { return 1 }
func (*stackFillGenerator) String() string  { return "stackFillGenerator" }
