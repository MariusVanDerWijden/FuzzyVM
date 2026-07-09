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
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
)

var callStrategies = []Strategy{
	new(createCallRNGGenerator),
	new(createCallGenerator),
	new(randomCallGenerator),
	new(callPrecompileGenerator),
}

type createCallRNGGenerator struct{}

func (*createCallRNGGenerator) Execute(env Environment) {
	// Create and call a random program
	var (
		code      = env.f.ByteSlice256()
		isCreate2 = env.f.Bool()
		callOp    = vm.OpCode(env.f.Byte())
	)
	env.CreateAndCall(code, isCreate2, callOp)
}

func (*createCallRNGGenerator) Importance() int {
	return 4
}

func (*createCallRNGGenerator) String() string {
	return "createCallRNGGenerator"
}

type createCallGenerator struct{}

func (*createCallGenerator) Execute(env Environment) {
	// Prevent too deep recursion. recursionLevel is per-generation, so this
	// only bounds the current call tree, not the whole fuzzer process.
	if env.recursionLevel >= maxRecursionLevel {
		return
	}
	// Create and call a meaningful program, generated one level deeper.
	var (
		seedLen   = env.f.Uint16()
		seed      = env.f.ByteSlice(int(seedLen))
		newFiller = filler.NewFiller(seed)
		_, code   = generateProgram(newFiller, env.recursionLevel+1)
		isCreate2 = env.f.Bool()
		callOp    = vm.OpCode(env.f.Byte())
	)
	env.CreateAndCall(code, isCreate2, callOp)
}

func (*createCallGenerator) Importance() int {
	return 5
}

func (*createCallGenerator) String() string {
	return "createCallGenerator"
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
		Gas:       uint256.MustFromBig(env.f.GasInt()),
		Address:   addr,
		Value:     env.f.BigInt16(),
		InOffset:  uint32(env.f.MemInt().Uint64()),
		InSize:    uint32(env.f.MemInt().Uint64()),
		OutOffset: uint32(env.f.MemInt().Uint64()),
		OutSize:   uint32(env.f.MemInt().Uint64()),
	}
	precompiles.CallRandomizer(env.p, env.f, c)
}

func (*randomCallGenerator) Importance() int {
	return 4
}

func (*randomCallGenerator) String() string {
	return "randomCallGenerator"
}

type callPrecompileGenerator struct{}

func (*callPrecompileGenerator) Execute(env Environment) {
	// Ignore errors: a failure to build the precompile input just means this
	// strategy contributes nothing to the program, not that anything crashed.
	_ = precompiles.CallPrecompile(env.p, env.f)
}

func (*callPrecompileGenerator) Importance() int {
	return 8
}

func (*callPrecompileGenerator) String() string {
	return "callPrecompileGenerator"
}
