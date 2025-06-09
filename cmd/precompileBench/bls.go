package main

import (
	"math/big"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fp"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
)

func makeBLSAdd(f *filler.Filler) []byte {
	a := getG1Point(f)
	b := getG1Point(f)
	p := program.New()
	p.Mstore(encodePointG1(a), 0)
	p.Mstore(encodePointG1(b), 128)
	_, dest := p.Jumpdest()
	p.StaticCall(nil, 0x0d, 0, 128, 0, 32)
	p.Op(vm.POP)
	p.Jump(dest)
	return p.Bytes()
}

func makeBLSMulExpG1(f *filler.Filler) []byte {
	rounds := f.Byte()

	p := program.New()
	for i := range rounds {
		a := getG1Point(f)
		p.Mstore(encodePointG1(a), uint32(i*160))
		p.Mstore(f.ByteSlice(32), uint32(i*160+128))
	}
	_, dest := p.Jumpdest()
	p.StaticCall(nil, 0x0c, 0, rounds*160, 0, 128)
	p.Op(vm.POP)
	p.Jump(dest)
	return p.Bytes()
}

func makeBLSAddG2(f *filler.Filler) []byte {
	a := getG2Point(f)
	b := getG2Point(f)
	p := program.New()
	p.Mstore(encodePointG2(a), 0)
	p.Mstore(encodePointG2(b), 256)
	_, dest := p.Jumpdest()
	p.StaticCall(nil, 0x0d, 0, 512, 0, 256)
	p.Op(vm.POP)
	p.Jump(dest)
	return p.Bytes()
}

func makeBLSMulExpG2(f *filler.Filler) []byte {
	rounds := f.Byte()

	p := program.New()
	for i := range int(rounds) {
		a := getG2Point(f)
		p.Mstore(encodePointG2(a), uint32(i)*288)
		p.Mstore(f.ByteSlice(32), uint32(i*288+128))
	}
	_, dest := p.Jumpdest()
	p.StaticCall(nil, 0x0e, 0, uint32(rounds)*288, 0, 128)
	p.Op(vm.POP)
	p.Jump(dest)
	return p.Bytes()
}

func makeBLSMapG1(f *filler.Filler) []byte {
	p := program.New()
	p.Mstore(f.ByteSlice(64), 0)
	_, dest := p.Jumpdest()
	p.StaticCall(nil, 0x10, 0, 64, 0, 128)
	p.Op(vm.POP)
	p.Jump(dest)
	return p.Bytes()
}

func makeBLSMapG2(f *filler.Filler) []byte {
	p := program.New()
	p.Mstore(f.ByteSlice(128), 0)
	_, dest := p.Jumpdest()
	p.StaticCall(nil, 0x11, 0, 512, 0, 256)
	p.Op(vm.POP)
	p.Jump(dest)
	return p.Bytes()
}

func makeBLSPairing(f *filler.Filler) []byte {
	var (
		rounds                   = f.Byte()
		curvePoints, twistPoints = pairingBLS(int(rounds), f)
		inSize                   = uint32(len(curvePoints) * 192)
		offset                   = uint32(0)
		p                        = program.New()
	)

	// Input to the precompile are a set of 64 bit bn256.G1 points and 128 bit bn256.G2 points.
	for i := range curvePoints {
		p.Mstore(curvePoints[i].Marshal(), offset)
		offset += 128
		p.Mstore(twistPoints[i].Marshal(), offset)
		offset += 256
	}

	_, dest := p.Jumpdest()
	p.StaticCall(nil, 0x0e, 0, inSize, inSize, inSize+64)
	p.Op(vm.POP)
	p.Jump(dest)
	return p.Bytes()
}

// encodePointG1 encodes a point into 128 bytes.
func encodePointG1(p *bls12381.G1Affine) []byte {
	out := make([]byte, 128)
	fp.BigEndian.PutElement((*[fp.Bytes]byte)(out[16:]), p.X)
	fp.BigEndian.PutElement((*[fp.Bytes]byte)(out[64+16:]), p.Y)
	return out
}

// encodePointG2 encodes a point into 256 bytes.
func encodePointG2(p *bls12381.G2Affine) []byte {
	out := make([]byte, 256)
	// encode x
	fp.BigEndian.PutElement((*[fp.Bytes]byte)(out[16:16+48]), p.X.A0)
	fp.BigEndian.PutElement((*[fp.Bytes]byte)(out[80:80+48]), p.X.A1)
	// encode y
	fp.BigEndian.PutElement((*[fp.Bytes]byte)(out[144:144+48]), p.Y.A0)
	fp.BigEndian.PutElement((*[fp.Bytes]byte)(out[208:208+48]), p.Y.A1)
	return out
}

func getG1Point(f *filler.Filler) *bls12381.G1Affine {

	b := f.ByteSlice(len(fp.Modulus().Bytes()))
	scalar := new(big.Int).SetBytes(b)

	// compute a random point
	cp := new(bls12381.G1Affine)
	_, _, g1Gen, _ := bls12381.Generators()
	cp.ScalarMultiplication(&g1Gen, scalar)
	return cp
}

func getG2Point(f *filler.Filler) *bls12381.G2Affine {

	b := f.ByteSlice(len(fp.Modulus().Bytes()))
	scalar := new(big.Int).SetBytes(b)

	// compute a random point
	cp := new(bls12381.G2Affine)
	_, _, _, g2Gen := bls12381.Generators()
	cp.ScalarMultiplication(&g2Gen, scalar)
	return cp
}

// pairingBLS sets up a (hopefully valid) pairingBLS.
// We try to create the following pairingBLS:
// e(aMul1 * G1, bMul1 * G2) * e(aMul2 * G1, bMul2 * G2) * ... * e(aMuln * G1, bMuln * G2) == e(G1, G2) ^ s
// with s = sum(x: 1 -> n: (aMulx * bMulx))
// This code is analogous to https://github.com/holiman/goevmlab/blob/master/fuzzing/bls12381.go
// Apparently it applies to barreto-naehrig curves too.
func pairingBLS(rounds int, f *filler.Filler) ([]*bls12381.G1Affine, []*bls12381.G2Affine) {
	var (
		curvePoints []*bls12381.G1Affine
		twistPoints []*bls12381.G2Affine
		target      = new(big.Int)
	)
	// LHS: sum(x: 1->n: e(aMulx * G1, bMulx * G2))
	for i := 0; i < int(rounds); i++ {
		// aMul * G1
		aMul := f.BigInt32()
		pointG1 := new(bls12381.G1Affine).ScalarMultiplication(new(bls12381.G1Affine), aMul)
		// bMul * G2
		bMul := f.BigInt32()
		pointG2 := new(bls12381.G2Affine).ScalarMultiplication(new(bls12381.G2Affine), bMul)
		// append to pairing
		curvePoints = append(curvePoints, pointG1)
		twistPoints = append(twistPoints, pointG2)
		// Add to s
		target = target.Add(target, aMul.Mul(aMul, bMul))
	}
	// RHS: e(G1, G2) ^ s
	pointG1 := new(bls12381.G1Affine).ScalarMultiplication(new(bls12381.G1Affine), target)
	pointG1 = pointG1.Neg(pointG1)
	pointG2 := new(bls12381.G2Affine).ScalarMultiplication(new(bls12381.G2Affine), big.NewInt(1))
	curvePoints = append(curvePoints, pointG1)
	twistPoints = append(twistPoints, pointG2)
	return curvePoints, twistPoints
}
