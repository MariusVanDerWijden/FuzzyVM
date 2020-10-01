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
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/holiman/goevmlab/fuzzing"
	"github.com/holiman/goevmlab/ops"
	"github.com/holiman/goevmlab/program"
)

var fork string
var sender = common.HexToAddress("a94f5374fce5edbc8e2a8697c15331677e6ebf0b")
var sk = hexutil.MustDecode("0x45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8")

func GenerateProgram(data []byte) (*fuzzing.GstMaker, []byte) {
	var (
		f        = filler.NewFiller(data)
		p        = program.NewProgram()
		jumpdest = uint64(0)
	)

	// Run for counter rounds
	counter := f.Byte()
	for i := 0; byte(i) < counter; i++ {
		rnd := f.Byte()
		switch rnd % 25 {
		case 0:
			// Just add a single opcode
			p.Op(ops.OpCode(f.Byte()))
		case 1:
			// Set a jumpdest
			jumpdest = p.Jumpdest()
		case 2:
			// Set a jumpdest label
			jumpdest = p.Label()
		case 3:
			// Set the jumpdest randomly
			jumpdest = f.Uint64()
		case 4:
			// Push the jumpdest on the stack
			p.Push(jumpdest)
		case 5:
			// Jump to a label (currently deactivated)
			// p.Jump(jumpdest)
		case 6:
			// Jump to a label (currently deactivated)
			// p.JumpIf(jumpdest, f.Bool())
		case 7:
			// Copy a part of memory into storage
			var (
				memStart  = int(f.Uint32())
				memSize   = int(f.Uint32())
				startSlot = int(f.Uint32())
			)
			p.MemToStorage(memStart, memSize, startSlot)
		case 8:
			// Store data into memory
			var (
				data     = f.ByteSlice256()
				memStart = f.Uint32()
			)
			p.Mstore(data, memStart)
		case 9:
			// Store data in storage (currently deactivated)
			/*
				var (
					data     = f.ByteSlice256()
					slot = f.Uint32()
				)
				p.Sstore(slot, data)
			*/
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
			// Create and call a meaningful program
			var (
				seedLen   = f.Uint32()
				seed      = f.ByteSlice(int(seedLen))
				_, code   = GenerateProgram(seed)
				isCreate2 = f.Bool()
				callOp    = ops.OpCode(f.Byte())
			)
			p.CreateAndCall(code, isCreate2, callOp)
		case 14:

		}
	}
	code := p.Bytecode()
	return createGstMaker(f, code), code
}

func createGstMaker(fill *filler.Filler, code []byte) *fuzzing.GstMaker {
	gst := fuzzing.NewGstMaker()
	gst.EnableFork(fork)
	// Add sender
	gst.AddAccount(sender, fuzzing.GenesisAccount{
		Nonce:   0,
		Balance: big.NewInt(0xffffffff),
		Storage: make(map[common.Hash]common.Hash),
		Code:    []byte{},
	})
	// Add code
	dest := common.HexToAddress("0x0000ca1100b1a7e")
	gst.AddAccount(dest, fuzzing.GenesisAccount{
		Code:    code,
		Balance: new(big.Int),
		Storage: make(map[common.Hash]common.Hash),
	})
	// Add the transaction
	tx := &fuzzing.StTransaction{
		GasLimit:   []uint64{12000000},
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
