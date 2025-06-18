package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/ethereum/go-ethereum/crypto"
)

func makeEcrecover(f *filler.Filler) []byte {
	sk, err := ecdsa.GenerateKey(crypto.S256(), f)
	if err != nil {
		panic(err)
	}
	hash := f.ByteSlice(32)
	sig, err := crypto.Sign(hash, sk)
	if err != nil {
		panic(err)
	}
	p := program.New()
	input := hash
	input = append(input, append(make([]byte, 31), sig[64]+27)...)
	input = append(input, sig[0:32]...)
	input = append(input, sig[32:64]...)
	p.Mstore(input, 0)
	_, dest := p.Jumpdest()
	p.StaticCall(nil, 0x1, 0, 128, 128, 32)
	p.Op(vm.POP)
	p.Jump(dest)
	return p.Bytes()
}

func makeR1Recover(f *filler.Filler) []byte {
	sk, err := ecdsa.GenerateKey(elliptic.P256(), f)
	if err != nil {
		panic(err)
	}
	hash := f.ByteSlice(32)
	r, s, err := ecdsa.Sign(rand.Reader, sk, hash)
	if err != nil {
		panic(err)
	}
	sig := hash
	sig = append(sig, r.Bytes()...)
	sig = append(sig, s.Bytes()...)
	sig = append(sig, sk.X.Bytes()...)
	sig = append(sig, sk.Y.Bytes()...)
	p := program.New()
	p.Mstore(sig, 0)
	_, dest := p.Jumpdest()
	p.StaticCall(nil, []byte{0x01, 00}, 0, 160, 160, 32)
	p.Op(vm.POP)
	p.Jump(dest)
	return p.Bytes()
}
