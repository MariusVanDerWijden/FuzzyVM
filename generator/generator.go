// Copyright 2020 Marius van der Wijden
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

// Package generator provides means to generate state tests for Ethereum.
package generator

import (
	"math/big"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/MariusVanDerWijden/FuzzyVM/generator/precompiles"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/holiman/goevmlab/fuzzing"
	"github.com/holiman/goevmlab/ops"
	"github.com/holiman/goevmlab/program"
)

var (
	fork              = "Istanbul"
	sender            = common.HexToAddress("a94f5374fce5edbc8e2a8697c15331677e6ebf0b")
	sk                = hexutil.MustDecode("0x45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8")
	recursionLevel    = 0
	maxRecursionLevel = 10
	minJumpDistance   = 10
)

// GenerateProgram creates a new evm program and returns
// a gstMaker based on it as well as its program code.
func GenerateProgram(f *filler.Filler) (*fuzzing.GstMaker, []byte) {
	var (
		p         = program.NewProgram()
		jumptable = NewJumptable(uint64(minJumpDistance))
	)

	// Run for counter rounds
	counter := f.Byte()
	for i := 0; i < int(counter); i++ {
		rnd := f.Byte()
		switch rnd % 25 {
		case 0:
			// Just add a single opcode
			op := ops.OpCode(f.Byte())
			// Nethermind currently uses a different blockhash provider in the statetests,
			// so ignore the blockhash operator to reduce false positives.
			// see: https://gist.github.com/MariusVanDerWijden/97fe9eb1aac074f7ccf6aef169aaadaa
			if op != ops.BLOCKHASH {
				p.Op(op)
			}
		case 1:
			// Set a jumpdest
			jumptable.Push(p.Jumpdest(), p.Label())
		case 2:
			// Set a jumpdest label
			jumptable.Push(p.Label(), p.Label())
		case 3:
			// Set the jumpdest randomly
			jumptable.Push(uint64(f.Uint16()), p.Label())
		case 4:
			// Push the jumpdest on the stack
			jumpdest := jumptable.Pop(p.Label())
			p.Push(jumpdest)
		case 5:
			// Jump to a label
			jumpdest := jumptable.Pop(p.Label())
			p.Jump(jumpdest)
		case 6:
			// Jumpi to a label
			var (
				jumpdest   = jumptable.Pop(p.Label())
				shouldJump = f.Bool()
				condition  = big.NewInt(0)
			)
			if shouldJump {
				condition = f.BigInt16()
			}
			// jumps if condition != 0
			p.JumpIf(jumpdest, condition)
		case 7:
			// Copy a part of memory into storage
			var (
				memStart  = int(f.Uint16())
				memSize   = int(f.Uint16())
				startSlot = int(f.Uint16())
			)
			// TODO MSTORE currently uses too much gas
			p.MemToStorage(memStart, memSize, startSlot)
		case 8:
			// Store data into memory
			var (
				data     = f.ByteSlice256()
				memStart = f.Uint32()
			)
			p.Mstore(data, memStart)
		case 9:
			// Store data in storage
			var (
				data = make([]byte, f.Byte()%32)
				slot = f.Uint32()
			)
			p.Sstore(slot, data)
		case 10:
			// Loads data into memory and returns it
			p.ReturnData(f.ByteSlice256())
		case 11:
			// Returns with offset, len
			var (
				offset = f.Uint32()
				len    = f.Uint32()
			)
			p.Return(offset, len)
		case 12:
			// Create and call a random program
			var (
				code      = f.ByteSlice256()
				isCreate2 = f.Bool()
				callOp    = ops.OpCode(f.Byte())
			)
			p.CreateAndCall(code, isCreate2, callOp)
		case 13:
			// Prevent to deep recursion
			if recursionLevel > maxRecursionLevel {
				continue
			}
			recursionLevel++
			// Create and call a meaningful program
			var (
				seedLen   = f.Uint16()
				seed      = f.ByteSlice(int(seedLen))
				newFiller = filler.NewFiller(seed)
				_, code   = GenerateProgram(newFiller)
				isCreate2 = f.Bool()
				callOp    = ops.OpCode(f.Byte())
			)
			p.CreateAndCall(code, isCreate2, callOp)
			// Decreasing recursion level generates to heavy test cases,
			// so once we reach maxRecursionLevel we don't create new CreateAndCalls.

			// recursionLevel--
		case 14:
			// Call a random address
			var addr common.Address
			if f.Bool() {
				// call a precompile
				addr = common.BigToAddress(new(big.Int).Mod(f.BigInt16(), big.NewInt(20)))
			} else {
				addr = common.BytesToAddress(f.ByteSlice(20))
			}

			c := precompiles.CallObj{
				Gas:       f.GasInt(),
				Address:   addr,
				Value:     f.BigInt16(),
				InOffset:  uint32(f.Uint16()),
				InSize:    uint32(f.Uint16()),
				OutOffset: uint32(f.Uint16()),
				OutSize:   uint32(f.Uint16()),
			}
			precompiles.CallRandomizer(p, f, c)
		case 15:
			precompiles.CallPrecompile(p, f)
		case 16:
			b := make([]byte, f.Byte()%32)
			p.Push(b)
		}
	}
	code := jumptable.InsertJumps(p.Bytecode())
	return createGstMaker(f, code), code
}

func createGstMaker(fill *filler.Filler, code []byte) *fuzzing.GstMaker {
	gst := fuzzing.NewGstMaker()
	gst.EnableFork(fork)
	// Add sender
	gst.AddAccount(sender, fuzzing.GenesisAccount{
		Nonce: 0,
		// Used to be 0xffffffff, increased to prevent sender to little money exceptions
		// see: https://gist.github.com/MariusVanDerWijden/008b91a61de4b0fb831b72c24600ef59
		Balance: big.NewInt(0x3fffffffffffffff),
		Storage: make(map[common.Hash]common.Hash),
		Code:    []byte{},
	})
	// Add code
	dest := common.HexToAddress("0x0000ca1100f022")
	gst.AddAccount(dest, fuzzing.GenesisAccount{
		Code:    code,
		Balance: new(big.Int),
		Storage: make(map[common.Hash]common.Hash),
	})
	// Add the transaction
	tx := &fuzzing.StTransaction{
		GasLimit:   []uint64{20000000},
		Nonce:      0,
		Value:      []string{randHex(fill, 4)},
		Data:       []string{randHex(fill, 100)},
		GasPrice:   big.NewInt(0x01),
		To:         dest.Hex(),
		PrivateKey: sk,
	}
	gst.SetTx(tx)
	return gst
}

func randHex(fill *filler.Filler, max int) string {
	return hexutil.Encode(fill.ByteSlice(max))
}
