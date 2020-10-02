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

// Package filler can fill objects based on a provided data source.
package filler

import (
	"encoding/binary"
	"math/big"
)

// Filler can be used to fill objects from a data source.
type Filler struct {
	data    []byte
	pointer int
	usedUp  bool
}

// NewFiller creates a new Filler.
func NewFiller(data []byte) *Filler {
	if len(data) == 0 {
		data = make([]byte, 1)
	}
	return &Filler{
		data:    data,
		pointer: 0,
		usedUp:  false,
	}
}

// incPointer increments the internal pointer
// to the next position to be read.
func (f *Filler) incPointer(i int) {
	if f.pointer+i >= len(f.data) {
		f.usedUp = true
	}
	f.pointer = (f.pointer + i) % len(f.data)
}

// Bool returns a new bool.
func (f *Filler) Bool() bool {
	b := f.Byte()
	return b > 127
}

// Byte returns a new byte.
func (f *Filler) Byte() byte {
	b := f.data[f.pointer]
	f.incPointer(1)
	return b
}

// Read implements the io.Reader interface.
func (f *Filler) Read(b []byte) (n int, err error) {
	// TODO (MariusVanDerWijden) this can be done more efficiently
	for i := 0; i < len(b); i++ {
		b[i] = f.Byte()
	}
	return len(b), nil
}

// BigInt returns a new big int in [0, 2^32)
func (f *Filler) BigInt() *big.Int {
	i := f.Uint32()
	return big.NewInt(int64(i))
}

// ByteSlice returns a byteslice with `items` values.
func (f *Filler) ByteSlice(items int) []byte {
	// TODO (MariusVanDerWijden) this can be done way more efficiently
	var b []byte
	for i := 0; i < items; i++ {
		b = append(b, f.Byte())
	}
	return b
}

// ByteSlice256 returns a byteslice with 1..256 values.
func (f *Filler) ByteSlice256() []byte {
	return f.ByteSlice(int(f.Byte()))
}

// Uint32 returns a new uint32.
func (f *Filler) Uint32() uint32 {
	return binary.BigEndian.Uint32(f.ByteSlice(4))
}

// Uint64 returns a new uint64.
func (f *Filler) Uint64() uint64 {
	return binary.BigEndian.Uint64(f.ByteSlice(8))
}

// Reset resets a filler.
func (f *Filler) Reset() {
	f.pointer = 0
	f.usedUp = false
}

// UsedUp returns wether all bytes from the source have been used.
func (f *Filler) UsedUp() bool {
	return f.usedUp
}
