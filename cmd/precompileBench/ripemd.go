package main

import (
	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/holiman/uint256"
)

func makeRipeMD(f *filler.Filler) []byte {
	data := f.ByteSlice(int(f.Uint16()))
	p := program.New()
	p.Mstore(data, 0)

	_, dest := p.Jumpdest()
	gas := 1000 + (len(data)+31)/32*120
	p.StaticCall(uint256.NewInt(uint64(gas)), 0x3, 0, len(data), 0, 32)
	p.Op(vm.POP)
	p.Jump(dest)
	p.Mstore(data, 0)
	return p.Bytes()
}

func makeDataCopy(f *filler.Filler) []byte {
	data := f.ByteSlice(int(f.Uint16()))
	p := program.New()
	p.Mstore(data, 0)

	_, dest := p.Jumpdest()
	gas := 100 + (len(data)+31)/32*3
	p.StaticCall(uint256.NewInt(uint64(gas)), 0x4, 0, len(data), 1, len(data)-1)
	p.Op(vm.POP)
	p.Jump(dest)
	p.Mstore(data, 0)
	return p.Bytes()
}
