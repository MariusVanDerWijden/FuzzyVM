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
	"github.com/MariusVanDerWijden/FuzzyVM/generator/precompiles"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/goevmlab/ops"
)

var basicStrategies = []Strategy{
	new(opcodeGenerator),
	new(jumpdestGenerator),
	new(jumpGenerator),
	new(memStorageGenerator),
	new(mstoreGenerator),
	new(sstoreGenerator),
	new(returnDataGenerator),
	new(returnGenerator),
	new(createCallRNGGenerator),
	new(createCallGenerator),
	new(randomCallGenerator),
	new(callPrecompileGenerator),
	new(pushGenerator),
}

type opcodeGenerator struct{}

func (*opcodeGenerator) Execute(env Environment) {
	// Just add a single opcode
	op := ops.OpCode(env.f.Byte())
	// Nethermind currently uses a different blockhash provider in the statetests,
	// so ignore the blockhash operator to reduce false positives.
	// see: https://gist.github.com/MariusVanDerWijden/97fe9eb1aac074f7ccf6aef169aaadaa
	if op != ops.BLOCKHASH {
		env.p.Op(op)
	}
}

func (*opcodeGenerator) Importance() int {
	return 10
}

type jumpdestGenerator struct{}

func (*jumpdestGenerator) Execute(env Environment) {
	switch env.f.Byte() % 10 {
	case 0:
		// Set a jumpdest label
		env.jumptable.Push(env.p.Label(), env.p.Label())
	case 1:
		// Set the jumpdest randomly
		env.jumptable.Push(uint64(env.f.Uint16()), env.p.Label())
	default:
		// Set a jumpdest
		env.jumptable.Push(env.p.Jumpdest(), env.p.Label())
	}
}

func (*jumpdestGenerator) Importance() int {
	return 5
}

type jumpGenerator struct{}

func (*jumpGenerator) Execute(env Environment) {
	if env.f.Bool() {
		// Jump to a label
		jumpdest := env.jumptable.Pop(env.p.Label())
		env.p.Jump(jumpdest)
	} else {
		// Jumpi to a label
		var (
			jumpdest   = env.jumptable.Pop(env.p.Label())
			shouldJump = env.f.Bool()
			condition  = big.NewInt(0)
		)
		if shouldJump {
			condition = env.f.BigInt()
		}
		// jumps if condition != 0
		env.p.JumpIf(jumpdest, condition)
	}
}

func (*jumpGenerator) Importance() int {
	return 7
}

type memStorageGenerator struct{}

func (*memStorageGenerator) Execute(env Environment) {
	// Copy a part of memory into storage
	var (
		memStart  = int(env.f.Uint16())
		memSize   = int(env.f.Uint16())
		startSlot = int(env.f.Uint16())
	)
	// TODO MSTORE currently uses too much gas
	env.p.MemToStorage(memStart, memSize, startSlot)
}

func (*memStorageGenerator) Importance() int {
	return 1
}

type mstoreGenerator struct{}

func (*mstoreGenerator) Execute(env Environment) {
	// Store data into memory
	var (
		data     = env.f.ByteSlice256()
		memStart = env.f.Uint32()
	)
	env.p.Mstore(data, memStart)
}

func (*mstoreGenerator) Importance() int {
	return 3
}

type sstoreGenerator struct{}

func (*sstoreGenerator) Execute(env Environment) {
	// Store data in storage
	var (
		data = make([]byte, env.f.Byte()%32)
		slot = env.f.Uint32()
	)
	env.p.Sstore(slot, data)
}

func (*sstoreGenerator) Importance() int {
	return 3
}

type returnDataGenerator struct{}

func (*returnDataGenerator) Execute(env Environment) {
	// Loads data into memory and returns it
	env.p.ReturnData(env.f.ByteSlice256())
}

func (*returnDataGenerator) Importance() int {
	return 1
}

type returnGenerator struct{}

func (*returnGenerator) Execute(env Environment) {
	// Returns with offset, len
	var (
		offset = uint32(env.f.Uint16())
		len    = uint32(env.f.Uint16())
	)
	env.p.Return(offset, len)
}

func (*returnGenerator) Importance() int {
	return 1
}

type createCallRNGGenerator struct{}

func (*createCallRNGGenerator) Execute(env Environment) {
	// Create and call a random program
	var (
		code      = env.f.ByteSlice256()
		isCreate2 = env.f.Bool()
		callOp    = ops.OpCode(env.f.Byte())
	)
	env.p.CreateAndCall(code, isCreate2, callOp)
}

func (*createCallRNGGenerator) Importance() int {
	return 4
}

type createCallGenerator struct{}

func (*createCallGenerator) Execute(env Environment) {
	// Prevent to deep recursion
	if recursionLevel > maxRecursionLevel {
		return
	}
	recursionLevel++
	// Create and call a meaningful program
	var (
		seedLen   = env.f.Uint16()
		seed      = env.f.ByteSlice(int(seedLen))
		newFiller = filler.NewFiller(seed)
		_, code   = GenerateProgram(newFiller)
		isCreate2 = env.f.Bool()
		callOp    = ops.OpCode(env.f.Byte())
	)
	env.p.CreateAndCall(code, isCreate2, callOp)
	// Decreasing recursion level generates to heavy test cases,
	// so once we reach maxRecursionLevel we don't create new CreateAndCalls.

	// recursionLevel--
}

func (*createCallGenerator) Importance() int {
	return 5
}

type randomCallGenerator struct{}

func (*randomCallGenerator) Execute(env Environment) {
	// Call a random address
	var addr common.Address
	if env.f.Bool() {
		// call a precompile
		addr = common.BigToAddress(new(big.Int).Mod(env.f.BigInt16(), big.NewInt(20)))
	} else {
		addr = common.BytesToAddress(env.f.ByteSlice(20))
	}

	c := precompiles.CallObj{
		Gas:       env.f.GasInt(),
		Address:   addr,
		Value:     env.f.BigInt16(),
		InOffset:  uint32(env.f.Uint16()),
		InSize:    uint32(env.f.Uint16()),
		OutOffset: uint32(env.f.Uint16()),
		OutSize:   uint32(env.f.Uint16()),
	}
	precompiles.CallRandomizer(env.p, env.f, c)
}

func (*randomCallGenerator) Importance() int {
	return 4
}

type callPrecompileGenerator struct{}

func (*callPrecompileGenerator) Execute(env Environment) {
	precompiles.CallPrecompile(env.p, env.f)
}

func (*callPrecompileGenerator) Importance() int {
	return 8
}

type pushGenerator struct{}

func (*pushGenerator) Execute(env Environment) {
	b := make([]byte, env.f.Byte()%32)
	env.p.Push(b)
}

func (*pushGenerator) Importance() int {
	return 4
}
