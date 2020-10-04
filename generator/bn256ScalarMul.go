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

package generator

import (
	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto/bn256"
	"github.com/holiman/goevmlab/program"
)

var bn256mulAddr = common.HexToAddress("0x7")

type bn256MulCaller struct{}

func (*bn256MulCaller) call(p *program.Program, f *filler.Filler) error {
	k := f.BigInt()
	point := new(bn256.G1).ScalarBaseMult(k)
	scalar := f.BigInt()
	c := callObj{
		gas:       f.BigInt(),
		address:   bn256mulAddr,
		inOffset:  0,
		inSize:    96,
		outOffset: 0,
		outSize:   64,
		value:     f.BigInt(),
	}
	// 64 bytes curve point
	p.Mstore(point.Marshal(), 0)
	// 32 bytes scalar
	bytes := make([]byte, 32)
	copy(bytes, scalar.Bytes())
	p.Mstore(bytes[:], 64)
	callRandomizer(p, f, c)
	return nil
}
