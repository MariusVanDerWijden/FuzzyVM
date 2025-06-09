package main

import (
	"github.com/ethereum/go-ethereum/common"
	"math/big"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/ethereum/go-ethereum/crypto/bn256"
	"github.com/holiman/uint256"
)

func createBN256Mul(f *filler.Filler) []byte {
	k := f.BigInt32()
	point := new(bn256.G1).ScalarBaseMult(k)

	scalarFactor := f.BigInt32()

	p := program.New()

	p.Mstore(point.Marshal(), 0)
	scalarBytes := common.LeftPadBytes(scalarFactor.Bytes(), 32)
	p.Mstore(scalarBytes, 64)

	_, dest := p.Jumpdest()
	p.StaticCall(uint256.NewInt(6000), 0x7, 0, 96, 0, 64)
	p.Op(vm.POP)
	p.Jump(dest)
	return p.Bytes()
}

func createBN256Add(f *filler.Filler) []byte {
	k := f.BigInt32()
	point := new(bn256.G1).ScalarBaseMult(k)
	k2 := f.BigInt32()
	point2 := new(bn256.G1).ScalarBaseMult(k2)

	p := program.New()
	p.Mstore(point.Marshal(), 0)
	p.Mstore(point2.Marshal(), 64)

	_, dest := p.Jumpdest()
	p.StaticCall(uint256.NewInt(150), 0x6, 0, 128, 0, 64)
	p.Op(vm.POP)
	p.Jump(dest)
	return p.Bytes()
}

func createBN256Pairing(f *filler.Filler) []byte {
	var (
		rounds                   = f.Byte()
		curvePoints, twistPoints = pairing(int(rounds), f)
		inSize                   = uint32(len(curvePoints) * 192)
		offset                   = uint32(0)
		p                        = program.New()
	)

	// Input to the precompile are a set of 64 bit bn256.G1 points and 128 bit bn256.G2 points.
	for i := range curvePoints {
		p.Mstore(curvePoints[i].Marshal(), offset)
		offset += 64
		p.Mstore(twistPoints[i].Marshal(), offset)
		offset += 128
	}

	_, dest := p.Jumpdest()
	p.StaticCall(nil, 0x8, 0, inSize, inSize, inSize+64)
	p.Op(vm.POP)
	p.Jump(dest)
	return p.Bytes()
}

// pairing sets up a (hopefully valid) pairing.
// We try to create the following pairing:
// e(aMul1 * G1, bMul1 * G2) * e(aMul2 * G1, bMul2 * G2) * ... * e(aMuln * G1, bMuln * G2) == e(G1, G2) ^ s
// with s = sum(x: 1 -> n: (aMulx * bMulx))
// This code is analogous to https://github.com/holiman/goevmlab/blob/master/fuzzing/bls12381.go
// Apparently it applies to barreto-naehrig curves too.
func pairing(rounds int, f *filler.Filler) ([]*bn256.G1, []*bn256.G2) {
	var (
		curvePoints []*bn256.G1
		twistPoints []*bn256.G2
		target      = new(big.Int)
	)
	// LHS: sum(x: 1->n: e(aMulx * G1, bMulx * G2))
	for i := 0; i < int(rounds); i++ {
		// aMul * G1
		aMul := f.BigInt32()
		pointG1 := new(bn256.G1).ScalarBaseMult(aMul)
		// bMul * G2
		bMul := f.BigInt32()
		pointG2 := new(bn256.G2).ScalarBaseMult(bMul)
		// append to pairing
		curvePoints = append(curvePoints, pointG1)
		twistPoints = append(twistPoints, pointG2)
		// Add to s
		target = target.Add(target, aMul.Mul(aMul, bMul))
	}
	// RHS: e(G1, G2) ^ s
	pointG1 := new(bn256.G1).ScalarBaseMult(target)
	pointG1 = pointG1.Neg(pointG1)
	pointG2 := new(bn256.G2).ScalarBaseMult(big.NewInt(1))
	curvePoints = append(curvePoints, pointG1)
	twistPoints = append(twistPoints, pointG2)
	return curvePoints, twistPoints
}
