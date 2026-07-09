package main

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	bn254 "github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/holiman/uint256"
)

// go-ethereum's crypto/bn256 moved to a gnark backend without ScalarBaseMult,
// so points are built with gnark-crypto and encoded in the raw EVM format the
// precompiles expect (big-endian coordinates, no compression flags).

func bnEncodeG1(p *bn254.G1Affine) []byte {
	out := make([]byte, 64)
	x := p.X.Bytes()
	y := p.Y.Bytes()
	copy(out[0:32], x[:])
	copy(out[32:64], y[:])
	return out
}

func bnEncodeG2(p *bn254.G2Affine) []byte {
	out := make([]byte, 128)
	xa1 := p.X.A1.Bytes()
	xa0 := p.X.A0.Bytes()
	ya1 := p.Y.A1.Bytes()
	ya0 := p.Y.A0.Bytes()
	copy(out[0:32], xa1[:])
	copy(out[32:64], xa0[:])
	copy(out[64:96], ya1[:])
	copy(out[96:128], ya0[:])
	return out
}

func createBN256Mul(f *filler.Filler) []byte {
	point := new(bn254.G1Affine).ScalarMultiplicationBase(f.BigInt32())

	scalarFactor := f.BigInt32()

	p := program.New()

	p.Mstore(bnEncodeG1(point), 0)
	scalarBytes := common.LeftPadBytes(scalarFactor.Bytes(), 32)
	p.Mstore(scalarBytes, 64)

	_, dest := p.Jumpdest()
	p.StaticCall(nil, 0x7, 0, 96, 0, 64)
	p.Op(vm.POP)
	p.Jump(dest)
	return p.Bytes()
}

func createBN256Add(f *filler.Filler) []byte {
	point := new(bn254.G1Affine).ScalarMultiplicationBase(f.BigInt32())
	point2 := new(bn254.G1Affine).ScalarMultiplicationBase(f.BigInt32())

	p := program.New()
	p.Mstore(bnEncodeG1(point), 0)
	p.Mstore(bnEncodeG1(point2), 64)

	_, dest := p.Jumpdest()
	p.StaticCall(uint256.NewInt(150), 0x6, 0, 128, 0, 64)
	p.Op(vm.POP)
	p.Jump(dest)
	return p.Bytes()
}

func createBN256Pairing(f *filler.Filler) []byte {
	var (
		rounds                   = f.Byte()
		curvePoints, twistPoints = pairingBn(int(rounds), f)
		inSize                   = uint32(len(curvePoints) * 192)
		offset                   = uint32(0)
		p                        = program.New()
	)

	// Input to the precompile are a set of 64 byte G1 points and 128 byte G2 points.
	for i := range curvePoints {
		p.Mstore(bnEncodeG1(curvePoints[i]), offset)
		offset += 64
		p.Mstore(bnEncodeG2(twistPoints[i]), offset)
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
func pairingBn(rounds int, f *filler.Filler) ([]*bn254.G1Affine, []*bn254.G2Affine) {
	var (
		curvePoints []*bn254.G1Affine
		twistPoints []*bn254.G2Affine
		target      = new(big.Int)
	)
	// LHS: sum(x: 1->n: e(aMulx * G1, bMulx * G2))
	for i := 0; i < int(rounds); i++ {
		// aMul * G1
		aMul := f.BigInt32()
		pointG1 := new(bn254.G1Affine).ScalarMultiplicationBase(aMul)
		// bMul * G2
		bMul := f.BigInt32()
		pointG2 := new(bn254.G2Affine).ScalarMultiplicationBase(bMul)
		// append to pairing
		curvePoints = append(curvePoints, pointG1)
		twistPoints = append(twistPoints, pointG2)
		// Add to s
		target = target.Add(target, aMul.Mul(aMul, bMul))
	}
	// RHS: e(G1, G2) ^ s
	pointG1 := new(bn254.G1Affine).ScalarMultiplicationBase(target)
	pointG1 = pointG1.Neg(pointG1)
	pointG2 := new(bn254.G2Affine).ScalarMultiplicationBase(big.NewInt(1))
	curvePoints = append(curvePoints, pointG1)
	twistPoints = append(twistPoints, pointG2)
	return curvePoints, twistPoints
}
