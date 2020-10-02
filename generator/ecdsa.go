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
	"crypto/ecdsa"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/goevmlab/program"
)

var ecdsaAddr = common.HexToAddress("0x1")

func callECDSA(p *program.Program, f *filler.Filler) error {
	sk, err := ecdsa.GenerateKey(crypto.S256().Params(), f)
	if err != nil {
		return err
	}
	sig, err := crypto.Sign(f.ByteSlice256(), sk)
	if err != nil {
		return err
	}
	p.Push(sig)
	c := callObj{
		gas:       f.BigInt(),
		address:   ecdsaAddr,
		inOffset:  0,
		inSize:    uint32(len(sig)),
		outOffset: 0,
		outSize:   20,
		value:     f.BigInt(),
	}
	callRandomizer(p, f, c)
	return nil
}
