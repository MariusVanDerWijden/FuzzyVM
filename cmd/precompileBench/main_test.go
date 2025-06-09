package main

import (
	"testing"

	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
)

func TestCreateStateTest(t *testing.T) {
	p := program.New()
	p.Jumpdest()
	p.Op(vm.GAS)
	p.Op(vm.EXTCODECOPY)
	p.Op(vm.JUMP)

	code := p.Bytes()
	writeOutTest(code, 0)
}
