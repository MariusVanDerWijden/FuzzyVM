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
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/core/vm"
)

// memOffset turns a filler MemInt into a non-negative int usable as a memory
// offset or length. MemInt occasionally returns values with the high bit set
// (up to 2^256); a plain int(x.Uint64()) conversion would wrap to a negative
// int on 64-bit platforms, which is never a valid offset. Clamp to a large but
// safe positive bound instead — anything that big just runs out of gas.
func memOffset(env Environment) int {
	v := env.f.MemInt()
	if !v.IsInt64() {
		return math.MaxInt32
	}
	i := v.Int64()
	if i < 0 || i > math.MaxInt32 {
		return math.MaxInt32
	}
	return int(i)
}

var basicStrategies = []Strategy{
	new(opcodeGenerator),
	new(memStorageGenerator),
	new(mstoreGenerator),
	new(sstoreGenerator),
	new(tstoreGenerator),
	new(returnDataGenerator),
	new(returnGenerator),
	new(pushGenerator),
	new(hashAndStoreGenerator),
	new(mloadGenerator),
	new(sloadGenerator),
	new(tloadGenerator),
	new(blobhashGenerator),
}

type opcodeGenerator struct{}

func (*opcodeGenerator) Execute(env Environment) {
	// Just add a single opcode
	op := vm.OpCode(env.f.Byte())
	env.p.Op(op)
}

func (*opcodeGenerator) Importance() int {
	return 10
}

func (*opcodeGenerator) String() string {
	return "opcodeGenerator"
}

// opcodeDefined[op] is true iff op is a defined EVM opcode. Built once at
// startup so the hot validOpcodeGenerator path is a single array lookup rather
// than formatting the opcode and substring-scanning for "not defined" (which
// also silently breaks if go-ethereum reworks OpCode.String()).
var opcodeDefined = func() [256]bool {
	var defined [256]bool
	for i := 0; i < 256; i++ {
		op := vm.OpCode(i)
		// String() returns "opcode 0x.. not defined" for unassigned opcodes.
		defined[i] = !strings.Contains(op.String(), "not defined")
	}
	return defined
}()

type validOpcodeGenerator struct{}

func (*validOpcodeGenerator) Execute(env Environment) {
	op := vm.OpCode(env.f.Byte())
	if !opcodeDefined[op] {
		// If the opcode is not defined, use JUMPDEST
		// since JUMPDEST is a valid opcode
		op = vm.JUMPDEST
	}
	env.p.Op(op)
}

func (*validOpcodeGenerator) Importance() int {
	return 10
}

func (*validOpcodeGenerator) String() string {
	return "validOpcodeGenerator"
}

type memStorageGenerator struct{}

func (*memStorageGenerator) Execute(env Environment) {
	// Copy a part of memory into storage
	var (
		memStart  = memOffset(env)
		memSize   = int(env.f.Byte())
		startSlot = memOffset(env)
	)
	// TODO MSTORE currently uses too much gas
	env.p.MemToStorage(memStart, memSize, startSlot)
}

func (*memStorageGenerator) Importance() int {
	return 1
}

func (*memStorageGenerator) String() string {
	return "memStorageGenerator"
}

type mstoreGenerator struct{}

func (*mstoreGenerator) Execute(env Environment) {
	// Store data into memory
	var (
		data     = env.f.ByteSlice256()
		memStart = uint32(env.f.MemInt().Uint64())
	)
	env.p.Mstore(data, memStart)
}

func (*mstoreGenerator) Importance() int {
	return 3
}

func (*mstoreGenerator) String() string {
	return "mstoreGenerator"
}

type sstoreGenerator struct{}

func (*sstoreGenerator) Execute(env Environment) {
	// Store a value in storage. Sometimes explicitly zero (to exercise the
	// clear/refund path), otherwise a boundary or random word so the slot
	// actually becomes dirty. Keeping the slot clustered in the low range (as
	// MemInt does) means a later SLOAD/SSTORE frequently hits the same warm slot.
	slot := uint32(env.f.MemInt().Uint64())
	env.p.Sstore(slot, storeValue(env))
}

// storeValue returns a value to write with SSTORE/TSTORE. ~1/8 of the time it is
// zero (so the clear/refund branches of the SSTORE gas machine are reachable);
// otherwise it is a boundary or random word so the slot actually becomes dirty.
func storeValue(env Environment) *big.Int {
	if env.f.Byte() < 32 {
		return big.NewInt(0)
	}
	return interestingOperand(env)
}

func (*sstoreGenerator) Importance() int {
	return 3
}

func (*sstoreGenerator) String() string {
	return "sstoreGenerator"
}

type tstoreGenerator struct{}

func (*tstoreGenerator) Execute(env Environment) {
	// Store a value in transient storage so a following TLOAD reads it back.
	slot := uint32(env.f.MemInt().Uint64())
	env.p.Tstore(slot, storeValue(env))
}

func (*tstoreGenerator) Importance() int {
	return 3
}

func (*tstoreGenerator) String() string {
	return "tstoreGenerator"
}

type returnDataGenerator struct{}

func (*returnDataGenerator) Execute(env Environment) {
	// Loads data into memory and returns it
	env.p.ReturnData(env.f.ByteSlice256())
}

func (*returnDataGenerator) Importance() int {
	return 1
}

func (*returnDataGenerator) String() string {
	return "returnDataGenerator"
}

type returnGenerator struct{}

func (*returnGenerator) Execute(env Environment) {
	// Returns with offset, len
	var (
		offset = memOffset(env)
		len    = memOffset(env)
	)
	env.p.Return(offset, len)
}

func (*returnGenerator) Importance() int {
	return 1
}

func (*returnGenerator) String() string {
	return "returnGenerator"
}

type pushGenerator struct{}

func (*pushGenerator) Execute(env Environment) {
	// Push a boundary or random word rather than the constant zero the old
	// make([]byte, n%32) idiom produced: a bare 0 on the stack rarely changes
	// which branch a following opcode takes, so it was mostly wasted weight.
	env.p.Push(interestingOperand(env))
}

func (*pushGenerator) Importance() int {
	return 4
}

func (*pushGenerator) String() string {
	return "pushGenerator"
}

type hashAndStoreGenerator struct{}

func (*hashAndStoreGenerator) Execute(env Environment) {
	env.p.Op(vm.RETURNDATASIZE)
	env.p.Push(0)
	env.p.Op(vm.MSIZE)
	env.p.Op(vm.RETURNDATACOPY)
	env.p.Op(vm.MSIZE)
	env.p.Push(0)
	env.p.Op(vm.KECCAK256)
	env.p.Op(vm.DUP1)
	env.p.Op(vm.SSTORE)
}

func (*hashAndStoreGenerator) Importance() int {
	return 2
}

func (*hashAndStoreGenerator) String() string {
	return "hashAndStoreGenerator"
}

type mloadGenerator struct{}

func (*mloadGenerator) Execute(env Environment) {
	offset := uint32(env.f.MemInt().Uint64())
	env.p.Push(offset)
	env.p.Op(vm.MLOAD)
}

func (*mloadGenerator) Importance() int {
	return 1
}

func (*mloadGenerator) String() string {
	return "mloadGenerator"
}

type sloadGenerator struct{}

func (*sloadGenerator) Execute(env Environment) {
	offset := uint32(env.f.MemInt().Uint64())
	env.p.Push(offset)
	env.p.Op(vm.SLOAD)
}

func (*sloadGenerator) Importance() int {
	return 1
}

func (*sloadGenerator) String() string {
	return "sloadGenerator"
}

type tloadGenerator struct{}

func (*tloadGenerator) Execute(env Environment) {
	offset := uint32(env.f.MemInt().Uint64())
	env.p.Push(offset)
	env.p.Op(vm.TLOAD)
}

func (*tloadGenerator) Importance() int {
	return 1
}

func (*tloadGenerator) String() string {
	return "tloadGenerator"
}

type blobhashGenerator struct{}

func (*blobhashGenerator) Execute(env Environment) {
	offset := uint32(env.f.MemInt().Uint64())
	env.p.Push(offset)
	env.p.Op(vm.BLOBHASH)
}

func (*blobhashGenerator) Importance() int {
	return 1
}

func (*blobhashGenerator) String() string {
	return "blobhashGenerator"
}
