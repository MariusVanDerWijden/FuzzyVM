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
	"fmt"
	"math/big"
	"os"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/goevmlab/fuzzing"
)

// Debug, when true, makes generateCode log every strategy it selects (and the
// resulting bytecode) to stdout, indented by recursion depth. Wired to the
// --debug flag on the fuzzyvm-db commands.
var Debug = false

var (
	fork              = "Amsterdam"
	sender            = common.HexToAddress("a94f5374fce5edbc8e2a8697c15331677e6ebf0b")
	sk                = hexutil.MustDecode("0x45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8")
	maxRecursionLevel = 10
)

var strategies *selector

func init() {
	strats := []Strategy{}
	strats = append(strats, basicStrategies...)
	strats = append(strats, callStrategies...)
	strats = append(strats, jumpStrategies...)
	strats = append(strats, stackStrategies...)
	strats = append(strats, coverageStrategies...)
	strategies = newSelector(strats)
}

// maxTotalBytes caps the total bytecode emitted across a whole generation tree
// (the top-level program plus every nested sub-generation). It is a budget
// shared by all recursion levels, not a per-level allowance, so a recursive
// program can't multiply it by its branching factor and depth.
const maxTotalBytes = 10000

// GenerateProgram creates a new evm program and returns
// a gstMaker based on it as well as its program code.
func GenerateProgram(f *filler.Filler) (*fuzzing.GstMaker, []byte) {
	budget := maxTotalBytes
	code := generateCode(f, 0, &budget)
	return CreateGstMaker(f, code), code
}

// generateCode builds the bytecode recursively, limited by a byte length budget.
func generateCode(f *filler.Filler, recursionLevel int, budget *int) []byte {
	if budget == nil {
		// Defensive: a direct caller (e.g. a test) may not supply one. Give this
		// subtree its own budget rather than dereferencing nil.
		b := maxTotalBytes
		budget = &b
	}
	var (
		labels      []uint64
		stackHeight int
		env         = Environment{
			p:              program.New(),
			f:              f,
			recursionLevel: recursionLevel,
			labels:         &labels,
			stackHeight:    &stackHeight,
			budget:         budget,
		}
	)

	// Run for counter rounds
	counter := f.Byte()
	prev := 0
	for range counter {
		// Stop as soon as the shared budget is exhausted — including by bytes
		// emitted in nested sub-generations this program spawned.
		if *budget <= 0 {
			break
		}
		// Select one of the strategies (weighted by Importance).
		strategy := strategies.Select(f)
		if Debug {
			fmt.Fprintf(os.Stderr, "%*sstrategy: %s\n", recursionLevel*2, "", strategy.String())
		}
		// Execute the strategy.
		strategy.Execute(env)
		grown := len(env.p.Bytes()) - prev
		prev = len(env.p.Bytes())
		*budget -= grown
	}
	code := env.p.Bytes()
	if Debug {
		fmt.Fprintf(os.Stderr, "%*sgenerated %d bytes: %x\n", recursionLevel*2, "", len(code), code)
	}
	return code
}

func CreateGstMaker(fill *filler.Filler, code []byte) *fuzzing.GstMaker {
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
		GasLimit:   []uint64{gasLimit(fill)},
		Nonce:      0,
		Value:      []string{randHex(fill, 4)},
		Data:       []string{randHex(fill, 100)},
		GasPrice:   big.NewInt(0x80),
		To:         dest.Hex(),
		PrivateKey: sk,
		Sender:     sender,
	}
	gst.SetTx(tx)
	return gst
}

// defaultGasLimit is the standard limit, capped at 16M.
const defaultGasLimit = uint64(params.MaxTxGas)

// gasLimit picks the transaction gas limit.
func gasLimit(f *filler.Filler) uint64 {
	switch b := f.Byte(); {
	case b < 200:
		// ~78%: run to completion.
		return defaultGasLimit
	case b < 240:
		// ~16%: a low limit that trips OOG early, exercising the cheap opcodes
		// and intrinsic-gas edge.
		return 21_000 + uint64(f.Uint16())
	default:
		// ~6%: somewhere in between, so OOG can land mid-program. Kept strictly
		// below defaultGasLimit so it never exceeds the EIP-7825 cap.
		return 21_000 + uint64(f.Uint32())%(defaultGasLimit-21_000)
	}
}

func randHex(fill *filler.Filler, max int) string {
	return hexutil.Encode(fill.ByteSlice(max))
}
