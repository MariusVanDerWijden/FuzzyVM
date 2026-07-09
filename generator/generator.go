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

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/goevmlab/fuzzing"
)

var (
	fork              = "Osaka"
	sender            = common.HexToAddress("a94f5374fce5edbc8e2a8697c15331677e6ebf0b")
	sk                = hexutil.MustDecode("0x45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8")
	maxRecursionLevel = 10
	minJumpDistance   = 10
)

var strategies = map[byte]Strategy{}

func init() {
	strats := []Strategy{}
	strats = append(strats, basicStrategies...)
	strats = append(strats, callStrategies...)
	strats = append(strats, jumpStrategies...)
	strategies = makeMap(strats)
}

// GenerateProgram creates a new evm program and returns
// a gstMaker based on it as well as its program code.
func GenerateProgram(f *filler.Filler) (*fuzzing.GstMaker, []byte) {
	return generateProgram(f, 0)
}

// generateProgram is the recursive core of GenerateProgram. recursionLevel is
// carried down through nested createCallGenerator invocations so the depth is
// bounded per top-level generation rather than by a process-global counter.
func generateProgram(f *filler.Filler, recursionLevel int) (*fuzzing.GstMaker, []byte) {
	var (
		labels []uint64
		env    = Environment{
			p:              program.New(),
			f:              f,
			jumptable:      NewJumptable(uint64(minJumpDistance)),
			recursionLevel: recursionLevel,
			labels:         &labels,
		}
		debug = false
	)

	// Run for counter rounds
	counter := f.Byte()
	for range counter {
		// Select one of the strategies
		rnd := f.Byte()
		strategy := strategies[rnd]
		if strategy == nil {
			panic(fmt.Sprintf("strategy %v is nil", rnd))
		}
		if debug {
			fmt.Println(rnd, strategy.String())
		}
		// Execute the strategy
		strategy.Execute(env)
		if len(env.p.Bytes()) > 10000 {
			break
		}
	}
	code := env.jumptable.InsertJumps(env.p.Bytes())
	if debug {
		fmt.Printf("length: %v \n%x\n", len(code), code)
	}
	return CreateGstMaker(f, code), code
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

// defaultGasLimit is the standard limit: as high as allowed so programs almost
// never run out of gas. Capped at the EIP-7825 per-transaction gas limit
// (params.MaxTxGas), which the Osaka fork enforces — exceeding it makes the
// state test unexecutable.
const defaultGasLimit = uint64(params.MaxTxGas)

// gasLimit picks the transaction gas limit. Most of the time it returns the
// generous default so programs run to completion, but a fraction of the time it
// returns a smaller limit so execution runs out of gas partway through. Gas
// accounting (memory-expansion cost, the CALL 63/64 rule, EIP-2929/3529
// warm/cold access and refunds, precompile gas formulas) is one of the most
// divergence-prone areas across clients, and it is only exercised when OOG
// actually lands mid-execution across the whole opcode range.
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
