package main

import (
	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
)

func makeSnippet(f *filler.Filler) []byte {
	p := program.New()
	_, dest := p.Jumpdest()
	p.Append(f.ByteSlice(int(f.Byte())))
	p.Jump(dest)
	return p.Bytes()
}

func makeMstore(f *filler.Filler) []byte {
	p := program.New()
	p.Mstore([]byte{0xff}, 7000000)
	p.Op(vm.GAS)
	_, dest := p.Jumpdest()
	p.Op(vm.MLOAD)
	p.Jump(dest)
	return p.Bytes()
}

func makeMstore2(f *filler.Filler) []byte {
	p := program.New()
	p.Push0()
	_, dest := p.Jumpdest()
	p.Push(1)
	p.Op(vm.ADD)
	p.Op(vm.DUP1)
	p.Op(vm.DUP1)
	p.Op(vm.MSTORE)
	p.Jump(dest)
	return p.Bytes()
}

func makeSStore(f *filler.Filler) []byte {
	p := program.New()
	_, dest := p.Jumpdest()
	p.Op(vm.ORIGIN)
	p.Op(vm.GAS)
	p.Op(vm.MUL)
	p.Op(vm.DUP1)
	p.Op(vm.SSTORE)
	p.Jump(dest)
	return p.Bytes()
}

func makeSdiv(f *filler.Filler) []byte {
	p := program.New()
	p.Push0()
	_, dest := p.Jumpdest()
	p.Push0()
	p.Op(vm.BLOCKHASH)
	p.Op(vm.SWAP1)
	p.Op(vm.SDIV)
	p.Jump(dest)
	return p.Bytes()
}

func makeSStore2(f *filler.Filler) []byte {
	p := program.New()
	p.Push0()
	p.Op(vm.SLOAD)
	_, dest := p.Jumpdest()
	p.Push(1)
	p.Op(vm.ADD)
	p.Op(vm.DUP1)
	p.Op(vm.DUP1)
	p.Op(vm.SSTORE)
	p.Op(vm.GAS)
	p.Push(80000)
	p.Op(vm.LT)
	p.Push(dest)
	p.Op(vm.JUMPI)
	p.Push0()
	p.Push0()
	p.Op(vm.SSTORE)
	return p.Bytes()
}
