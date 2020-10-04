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

package precompiles

import (
	"math/big"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto/bn256"
	"github.com/holiman/goevmlab/program"
)

var bn256pairingAddr = common.HexToAddress("0x8")

type bn256PairingCaller struct{}

func (*bn256PairingCaller) call(p *program.Program, f *filler.Filler) error {
	var (
		rounds                   = f.Byte()
		curvePoints, twistPoints = pairing(int(rounds), f)
		inSize                   = uint32(len(curvePoints) * 192)
		offset                   = uint32(0)
	)

	c := CallObj{
		Gas:       f.BigInt(),
		Address:   bn256pairingAddr,
		InOffset:  0,
		InSize:    inSize,
		OutOffset: 0,
		OutSize:   64,
		Value:     f.BigInt(),
	}
	// Input to the precompile are a set of 64 bit bn256.G1 points and 128 bit bn256.G2 points.
	for i := range curvePoints {
		p.Mstore(curvePoints[i].Marshal(), offset)
		offset += 64
		p.Mstore(twistPoints[i].Marshal(), offset)
		offset += 128
	}
	CallRandomizer(p, f, c)
	return nil
}

// pairing sets up a (hopefully valid) pairing.
// We try to create the following pairing:
// e(aMul1 * G1, bMul1 * G2) * e(aMul2 * G1, bMul2 * G2) * ... * e(aMuln * G1, bMuln * G2) == e(G1, G2) ^ s
// with s = sum(x: 1 -> n: (aMulx * bMulx))
// This code is analogous to https://github.com/holiman/goevmlab/blob/master/fuzzing/bls12381.go
// But I'm not sure if it applies to barreto-naehrig curves too.
func pairing(rounds int, f *filler.Filler) ([]*bn256.G1, []*bn256.G2) {
	var (
		curvePoints []*bn256.G1
		twistPoints []*bn256.G2
		target      *big.Int
	)
	// LHS: sum(x: 1->n: e(aMulx * G1, bMulx * G2))
	for i := 0; i < int(rounds); i++ {
		// aMul * G1
		aMul := f.BigInt()
		pointG1 := new(bn256.G1).ScalarBaseMult(aMul)
		// bMul * G2
		bMul := f.BigInt()
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
	pointG2 := new(bn256.G2)
	curvePoints = append(curvePoints, pointG1)
	twistPoints = append(twistPoints, pointG2)
	return curvePoints, twistPoints
}
