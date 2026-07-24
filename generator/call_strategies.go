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

// callOps are the opcodes that actually consume the call frame CreateAndCall
// builds. Drawing the op from a raw filler byte instead (the previous behaviour)
// hit one of these only 4/256 = 1.5% of the time: the other 98.5% emitted an
// unrelated — often undefined — opcode, so the whole create-and-call setup (and,
// for createCallGenerator, a full recursive sub-generation) was wasted.
var callOps = []vm.OpCode{vm.CALL, vm.CALLCODE, vm.DELEGATECALL, vm.STATICCALL}

// randomCallOp picks the opcode used to invoke a freshly created contract.
// Usually a real call op so the call actually happens; occasionally (~1/32) a
// raw random byte, which keeps a little coverage of "operands pushed, then
// something else executes" without wasting nearly every invocation on it.
func randomCallOp(env Environment) vm.OpCode {
	if env.f.Byte() < 8 {
		return vm.OpCode(env.f.Byte())
	}
	return callOps[int(env.f.Byte())%len(callOps)]
}

type createCallRNGGenerator struct{}

func (*createCallRNGGenerator) Execute(env Environment) {
	// Create and call a random program
	var (
		code      = env.f.ByteSlice256()
		isCreate2 = env.f.Bool()
		callOp    = randomCallOp(env)
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
	// Deploy and call a meaningful program, generated one level deeper.
	var (
		seedLen   = env.f.Uint16()
		seed      = env.f.ByteSlice(int(seedLen))
		newFiller = filler.NewFiller(seed)
		code      = generateCode(newFiller, env.recursionLevel+1, env.budget)
		isCreate2 = env.f.Bool()
		callOp    = randomCallOp(env)
	)
	env.DeployAndCall(code, isCreate2, callOp)
}

func (*createCallGenerator) Importance() int {
	return 5
}

func (*createCallGenerator) String() string {
	return "createCallGenerator"
}

type randomCallGenerator struct{}

// precompileAddrs are the addresses to bias random calls toward. Covers the
// low-numbered precompiles (0x01..0x11) and P256VERIFY at 0x0100, which the old
// Mod(_, 20) range (0x00..0x13) could not reach — and which also included the
// non-precompile addresses 0x00, 0x12, 0x13.
var precompileAddrs = []common.Address{
	common.HexToAddress("0x01"), common.HexToAddress("0x02"),
	common.HexToAddress("0x03"), common.HexToAddress("0x04"),
	common.HexToAddress("0x05"), common.HexToAddress("0x06"),
	common.HexToAddress("0x07"), common.HexToAddress("0x08"),
	common.HexToAddress("0x09"), common.HexToAddress("0x0a"),
	common.HexToAddress("0x0b"), common.HexToAddress("0x0c"),
	common.HexToAddress("0x0d"), common.HexToAddress("0x0e"),
	common.HexToAddress("0x0f"), common.HexToAddress("0x10"),
	common.HexToAddress("0x11"), common.HexToAddress("0x0100"),
}

func (*randomCallGenerator) Execute(env Environment) {
	// Call a random address
	var addr common.Address
	if env.f.Bool() {
		// call a precompile
		addr = precompileAddrs[int(env.f.Byte())%len(precompileAddrs)]
	} else {
		addr = common.BytesToAddress(env.f.ByteSlice(20))
	}

	// Do some gas > u64 every know and then.
	gas := uint256.MustFromBig(env.f.GasInt())
	if env.f.Byte() < 8 { // ~1/32
		gas = new(uint256.Int).Lsh(uint256.NewInt(1), 64+uint(env.f.Byte())%192)
	}
	c := precompiles.CallObj{
		Gas:       gas,
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
