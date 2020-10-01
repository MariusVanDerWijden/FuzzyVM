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

	"github.com/mariusvanderwijden/fuzzyvm/filler"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/holiman/goevmlab/fuzzing"
)

var fork string
var sender = common.HexToAddress("a94f5374fce5edbc8e2a8697c15331677e6ebf0b")
var sk = hexutil.MustDecode("0x45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8")

func GenerateProgram(data []byte) *fuzzing.GstMaker {
	return nil
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

func randHex(fill filler.Filler, max int) string {
	return hexutil.Encode(fill.ByteSlice(max))
}
