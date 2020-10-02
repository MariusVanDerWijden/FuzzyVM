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
	"github.com/holiman/goevmlab/program"
)

var sha256Addr = common.HexToAddress("0x2")

type sha256Caller struct{}

func (*sha256Caller) call(p *program.Program, f *filler.Filler) error {
	data := f.ByteSlice(int(f.Uint32()))
	c := callObj{
		gas:       f.BigInt(),
		address:   sha256Addr,
		inOffset:  0,
		inSize:    uint32(len(data)),
		outOffset: 0,
		outSize:   32,
		value:     f.BigInt(),
	}
	p.Push(data)
	callRandomizer(p, f, c)
	return nil
}
