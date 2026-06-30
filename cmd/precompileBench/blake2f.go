package main

import (
	"encoding/binary"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
)

func makeBlake2f(f *filler.Filler) []byte {
	var (
		input  = make([]byte, 213)
		offset = 0
	)
	// Rounds
	rounds := uint32(0x0ffffff) //f.Uint32()
	binary.BigEndian.PutUint32(input, rounds)
	offset += 4
	// h
	for i := 0; i < 8; i++ {
		binary.BigEndian.PutUint64(input[offset:], f.Uint64())
		offset += 8
	}
	// m
	for i := 0; i < 16; i++ {
		binary.BigEndian.PutUint64(input[offset:], f.Uint64())
		offset += 8
	}
	// t
	for i := 0; i < 2; i++ {
		binary.BigEndian.PutUint64(input[offset:], f.Uint64())
		offset += 8
	}
	// Valid or invalid inputs
	if f.Bool() {
		input[212] = f.Byte()
	} else {
		if f.Bool() {
			input[212] = 0
		} else {
			input[212] = 1
		}
	}

	p := program.New()
	p.Mstore(input, 0)

	_, dest := p.Jumpdest()
	p.StaticCall(nil, 0x9, 0, 213, 213, 64)
	p.Op(vm.POP)
	p.Jump(dest)
	return p.Bytes()
}
