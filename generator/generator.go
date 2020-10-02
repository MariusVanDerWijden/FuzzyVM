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

type precompile interface {
	call(p *program.Program, f *filler.Filler) error
}

var (
	fork   = "Istanbul"
	sender = common.HexToAddress("a94f5374fce5edbc8e2a8697c15331677e6ebf0b")
	sk     = hexutil.MustDecode("0x45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8")

	precompiles = []precompile{
		new(ecdsaCaller),
		new(sha256Caller),
		new(ripemdCaller),
		new(identityCaller),
	}
)

// GenerateProgram creates a new evm program and returns
// a gstMaker based on it as well as its program code.
func GenerateProgram(f *filler.Filler) (*fuzzing.GstMaker, []byte) {
	var (
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
			op := ops.OpCode(f.Byte())
			// Nethermind currently uses a different blockhash provider in the statetests,
			// so ignore the blockhash operator to reduce false positives.
			// see: https://gist.github.com/MariusVanDerWijden/97fe9eb1aac074f7ccf6aef169aaadaa
			if op != ops.BLOCKHASH {
				p.Op(op)
			}
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
				newFiller = filler.NewFiller(seed)
				_, code   = GenerateProgram(newFiller)
				isCreate2 = f.Bool()
				callOp    = ops.OpCode(f.Byte())
			)
			p.CreateAndCall(code, isCreate2, callOp)
		case 14:
			// Call a random address
			c := callObj{
				gas:       f.BigInt(),
				address:   common.BytesToAddress(f.ByteSlice(20)),
				value:     f.BigInt(),
				inOffset:  f.Uint32(),
				inSize:    f.Uint32(),
				outOffset: f.Uint32(),
				outSize:   f.Uint32(),
			}
			callRandomizer(p, f, c)
		case 15:
			// call a precompile
			var (
				idx  = int(f.Byte()) & len(precompiles)
				prec = precompiles[idx]
			)
			if err := prec.call(p, f); err != nil {
				panic(err)
			}
		}
	}
	code := p.Bytecode()
	return createGstMaker(f, code), code
}

type callObj struct {
	gas       *big.Int
	address   common.Address
	value     *big.Int
	inOffset  uint32
	inSize    uint32
	outOffset uint32
	outSize   uint32
}

func callRandomizer(p *program.Program, f *filler.Filler, c callObj) {
	switch f.Byte() % 3 {
	case 0:
		p.Call(c.gas, c.address, c.value, c.inOffset, c.inSize, c.outOffset, c.outSize)
	case 1:
		p.CallCode(c.gas, c.address, c.value, c.inOffset, c.inSize, c.outOffset, c.outSize)
	case 2:
		p.StaticCall(c.gas, c.address, c.inOffset, c.inSize, c.outOffset, c.outSize)
	}
}

func createGstMaker(fill *filler.Filler, code []byte) *fuzzing.GstMaker {
	gst := fuzzing.NewGstMaker()
	gst.EnableFork(fork)
	// Add sender
	gst.AddAccount(sender, fuzzing.GenesisAccount{
		Nonce: 0,
		// Used to be 0xffffffff, increased to prevent sender to little money exceptions
		// see: https://gist.github.com/MariusVanDerWijden/008b91a61de4b0fb831b72c24600ef59
		Balance: big.NewInt(0xffffffffffffff),
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
