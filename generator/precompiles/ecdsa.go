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

package precompiles

import (
	"crypto/ecdsa"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

var ecdsaAddr = common.HexToAddress("0x1")

type ecdsaCaller struct{}

func (*ecdsaCaller) call(p *program.Program, f *filler.Filler) error {
	sk, err := ecdsa.GenerateKey(crypto.S256(), f)
	if err != nil {
		return err
	}
	hash := f.ByteSlice(32)
	sig, err := crypto.Sign(hash, sk)
	if err != nil {
		return err
	}
	input := make([]byte, 128)
	copy(input[0:32], hash)         // signed message hash
	input[63] = sig[64] + 27        // v
	copy(input[64:96], sig[0:32])   // r
	copy(input[96:128], sig[32:64]) // s
	c := CallObj{
		Gas:       uint256.MustFromBig(f.GasInt()),
		Address:   ecdsaAddr,
		InOffset:  0,
		InSize:    uint32(len(input)),
		OutOffset: 0,
		OutSize:   32,
		Value:     f.BigInt32(),
	}
	p.Mstore(input, 0)
	CallRandomizer(p, f, c)
	return nil
}
