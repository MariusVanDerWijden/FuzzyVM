package main

import (
	"crypto/ecdsa"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

func makeEcrecover(f *filler.Filler) []byte {
	sk, err := ecdsa.GenerateKey(crypto.S256(), f)
	if err != nil {
		panic(err)
	}
	sig, err := crypto.Sign(f.ByteSlice(32), sk)
	if err != nil {
		panic(err)
	}
	p := program.New()
	p.Mstore(sig, 0)
	_, dest := p.Jumpdest()
	p.StaticCall(uint256.NewInt(0xf00), 0x1, 0, 128, 0, 32)
	p.Op(vm.POP)
	p.Jump(dest)
	return p.Bytes()
}
