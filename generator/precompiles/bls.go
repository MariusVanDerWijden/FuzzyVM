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
	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fp"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/holiman/uint256"
)

// EIP-2537 BLS12-381 precompile addresses (active from Prague).
var (
	blsG1AddAddr   = common.HexToAddress("0x0b")
	blsG1MSMAddr   = common.HexToAddress("0x0c")
	blsG2AddAddr   = common.HexToAddress("0x0d")
	blsG2MSMAddr   = common.HexToAddress("0x0e")
	blsPairingAddr = common.HexToAddress("0x0f")
	blsMapG1Addr   = common.HexToAddress("0x10")
	blsMapG2Addr   = common.HexToAddress("0x11")
)

// EIP-2537 encoded element/point sizes.
const (
	blsG1Size     = 128 // 2 * 64-byte field elements
	blsG2Size     = 256 // 4 * 64-byte field elements
	blsScalarSize = 32
	blsG1MSMItem  = blsG1Size + blsScalarSize // 160
	blsG2MSMItem  = blsG2Size + blsScalarSize // 288
	blsPairItem   = blsG1Size + blsG2Size     // 384
)

// --- callers ---

type blsG1AddCaller struct{}

func (*blsG1AddCaller) call(p *program.Program, f *filler.Filler) error {
	p.Mstore(encodeBLSG1(randG1(f)), 0)
	p.Mstore(encodeBLSG1(randG1(f)), uint32(blsG1Size))
	callBLS(p, f, blsG1AddAddr, 2*blsG1Size, blsG1Size)
	return nil
}

type blsG2AddCaller struct{}

func (*blsG2AddCaller) call(p *program.Program, f *filler.Filler) error {
	p.Mstore(encodeBLSG2(randG2(f)), 0)
	p.Mstore(encodeBLSG2(randG2(f)), uint32(blsG2Size))
	callBLS(p, f, blsG2AddAddr, 2*blsG2Size, blsG2Size)
	return nil
}

type blsG1MSMCaller struct{}

func (*blsG1MSMCaller) call(p *program.Program, f *filler.Filler) error {
	rounds := blsRounds(f, 8)
	for i := 0; i < rounds; i++ {
		p.Mstore(encodeBLSG1(randG1(f)), uint32(i*blsG1MSMItem))
		p.Mstore(scalar32(f), uint32(i*blsG1MSMItem+blsG1Size))
	}
	callBLS(p, f, blsG1MSMAddr, uint32(rounds*blsG1MSMItem), blsG1Size)
	return nil
}

type blsG2MSMCaller struct{}

func (*blsG2MSMCaller) call(p *program.Program, f *filler.Filler) error {
	rounds := blsRounds(f, 8)
	for i := 0; i < rounds; i++ {
		p.Mstore(encodeBLSG2(randG2(f)), uint32(i*blsG2MSMItem))
		p.Mstore(scalar32(f), uint32(i*blsG2MSMItem+blsG2Size))
	}
	callBLS(p, f, blsG2MSMAddr, uint32(rounds*blsG2MSMItem), blsG2Size)
	return nil
}

type blsMapG1Caller struct{}

func (*blsMapG1Caller) call(p *program.Program, f *filler.Filler) error {
	// A single 64-byte field element.
	p.Mstore(encodeBLSFp(f), 0)
	callBLS(p, f, blsMapG1Addr, 64, blsG1Size)
	return nil
}

type blsMapG2Caller struct{}

func (*blsMapG2Caller) call(p *program.Program, f *filler.Filler) error {
	// Two 64-byte field elements (a Fp2 element).
	p.Mstore(encodeBLSFp(f), 0)
	p.Mstore(encodeBLSFp(f), 64)
	callBLS(p, f, blsMapG2Addr, 128, blsG2Size)
	return nil
}

type blsPairingCaller struct{}

func (*blsPairingCaller) call(p *program.Program, f *filler.Filler) error {
	g1s, g2s := blsPairing(blsRounds(f, 4), f)
	offset := uint32(0)
	for i := range g1s {
		p.Mstore(encodeBLSG1(g1s[i]), offset)
		offset += blsG1Size
		p.Mstore(encodeBLSG2(g2s[i]), offset)
		offset += blsG2Size
	}
	callBLS(p, f, blsPairingAddr, uint32(len(g1s)*blsPairItem), 32)
	return nil
}

// Points that are on the curve but NOT in the prime-order subgroup.
var (
	blsG1NotInSubgroup = common.Hex2Bytes("000000000000000000000000000000000123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef00000000000000000000000000000000193fb7cedb32b2c3adc06ec11a96bc0d661869316f5e4a577a9f7c179593987beb4fb2ee424dbb2f5dd891e228b46c4a")
	blsG2NotInSubgroup = common.Hex2Bytes("00000000000000000000000000000000197bfd0342bbc8bee2beced2f173e1a87be576379b343e93232d6cef98d84b1d696e5612ff283ce2cfdccb2cfb65fa0c00000000000000000000000000000000184e811f55e6f9d84d77d2f79102fd7ea7422f4759df5bf7f6331d550245e3f1bcf6a30e3b29110d85e0ca16f9f6ae7a000000000000000000000000000000000f10e1eb3c1e53d2ad9cf2d398b2dc22c5842fab0a74b174f691a7e914975da3564d835cd7d2982815b8ac57f507348f000000000000000000000000000000000767d1c453890f1b9110fda82f5815c27281aba3f026ee868e4176a0654feea41a96575e0c4d58a14dbfbcc05b5010b1")
)

// blsSubgroupCaller feeds an on-curve-but-not-in-subgroup point into a BLS
// precompile that applies subgroup checks (G1-MSM, G2-MSM, pairing), hitting the
// otherwise-dark rejection paths. Each invocation targets one of the four
// rejection sites; the pairing G2 case uses a valid G1 so the G1 check passes
// first and the G2 check is actually reached.
type blsSubgroupCaller struct{}

func (*blsSubgroupCaller) call(p *program.Program, f *filler.Filler) error {
	switch f.Byte() % 4 {
	case 0: // G1-MSM, bad G1 -> errBLS12381G1PointSubgroup
		p.Mstore(blsG1NotInSubgroup, 0)
		p.Mstore(scalar32(f), blsG1Size)
		callBLS(p, f, blsG1MSMAddr, blsG1MSMItem, blsG1Size)
	case 1: // G2-MSM, bad G2 -> errBLS12381G2PointSubgroup
		p.Mstore(blsG2NotInSubgroup, 0)
		p.Mstore(scalar32(f), blsG2Size)
		callBLS(p, f, blsG2MSMAddr, blsG2MSMItem, blsG2Size)
	case 2: // pairing, bad G1 (valid G2) -> G1 subgroup rejection
		p.Mstore(blsG1NotInSubgroup, 0)
		p.Mstore(encodeBLSG2(randG2(f)), blsG1Size)
		callBLS(p, f, blsPairingAddr, blsPairItem, 32)
	default: // pairing, valid G1 + bad G2 -> G2 subgroup rejection
		p.Mstore(encodeBLSG1(randG1(f)), 0)
		p.Mstore(blsG2NotInSubgroup, blsG1Size)
		callBLS(p, f, blsPairingAddr, blsPairItem, 32)
	}
	return nil
}

// blsRounds picks how many (expensive) elliptic-curve rounds an MSM/pairing
// builder does. Each round is a full ScalarMultiplication, the dominant cost in
// generation, so bias hard toward a single round and only occasionally (~1/8)
// draw a larger count up to maxRounds. This keeps the heavy builders from
// halving generation throughput while still exercising multi-element inputs.
func blsRounds(f *filler.Filler, maxRounds int) int {
	if f.Byte() < 32 {
		return int(f.Byte())%maxRounds + 1
	}
	return 1
}

// callBLS issues the STATICCALL-style precompile call. All BLS precompiles are
// pure, so a value transfer is meaningless; CallRandomizer may still turn it
// into CALL/CALLCODE, which just wastes gas — acceptable for fuzzing.
func callBLS(p *program.Program, f *filler.Filler, addr common.Address, inSize, outSize uint32) {
	c := CallObj{
		Gas:       uint256.MustFromBig(f.GasInt()),
		Address:   addr,
		InOffset:  0,
		InSize:    inSize,
		OutOffset: 0,
		OutSize:   outSize,
		Value:     f.BigInt32(),
	}
	CallRandomizer(p, f, c)
}

func randG1(f *filler.Filler) *bls12381.G1Affine {
	s := new(big.Int).SetBytes(f.ByteSlice(fp.Bytes))
	_, _, g1, _ := bls12381.Generators()
	return new(bls12381.G1Affine).ScalarMultiplication(&g1, s)
}

func randG2(f *filler.Filler) *bls12381.G2Affine {
	s := new(big.Int).SetBytes(f.ByteSlice(fp.Bytes))
	_, _, _, g2 := bls12381.Generators()
	return new(bls12381.G2Affine).ScalarMultiplication(&g2, s)
}

// encodeBLSG1 encodes a G1 point as 128 bytes: two 16-byte-left-padded 48-byte
// coordinates, matching EIP-2537 / go-ethereum's decodePointG1.
func encodeBLSG1(p *bls12381.G1Affine) []byte {
	out := make([]byte, blsG1Size)
	fp.BigEndian.PutElement((*[fp.Bytes]byte)(out[16:64]), p.X)
	fp.BigEndian.PutElement((*[fp.Bytes]byte)(out[80:128]), p.Y)
	return out
}

// encodeBLSG2 encodes a G2 point as 256 bytes: X.A0, X.A1, Y.A0, Y.A1, each a
// 16-byte-left-padded 48-byte coordinate, matching go-ethereum's encodePointG2.
func encodeBLSG2(p *bls12381.G2Affine) []byte {
	out := make([]byte, blsG2Size)
	fp.BigEndian.PutElement((*[fp.Bytes]byte)(out[16:64]), p.X.A0)
	fp.BigEndian.PutElement((*[fp.Bytes]byte)(out[80:128]), p.X.A1)
	fp.BigEndian.PutElement((*[fp.Bytes]byte)(out[144:192]), p.Y.A0)
	fp.BigEndian.PutElement((*[fp.Bytes]byte)(out[208:256]), p.Y.A1)
	return out
}

// encodeBLSFp returns a 64-byte encoded field element (16 zero bytes + 48-byte
// value) that is guaranteed to be below the modulus, as MapToCurve requires.
func encodeBLSFp(f *filler.Filler) []byte {
	var e fp.Element
	e.SetBytes(f.ByteSlice(fp.Bytes))
	out := make([]byte, 64)
	fp.BigEndian.PutElement((*[fp.Bytes]byte)(out[16:64]), e)
	return out
}

// scalar32 returns a 32-byte scalar for MSM.
func scalar32(f *filler.Filler) []byte {
	b := make([]byte, blsScalarSize)
	copy(b, f.ByteSlice(blsScalarSize))
	return b
}

// blsPairing builds a (hopefully valid) pairing set that satisfies
// e(a_i*G1, b_i*G2) product == identity, analogous to the bn256 case.
func blsPairing(rounds int, f *filler.Filler) ([]*bls12381.G1Affine, []*bls12381.G2Affine) {
	var (
		g1s        []*bls12381.G1Affine
		g2s        []*bls12381.G2Affine
		target     = new(big.Int)
		_, _, a, b = bls12381.Generators()
	)
	for i := 0; i < rounds; i++ {
		aMul := f.BigInt32()
		g1s = append(g1s, new(bls12381.G1Affine).ScalarMultiplication(&a, aMul))
		bMul := f.BigInt32()
		g2s = append(g2s, new(bls12381.G2Affine).ScalarMultiplication(&b, bMul))
		target.Add(target, new(big.Int).Mul(aMul, bMul))
	}
	// RHS: -( s * e(G1, G2) ) so the whole product is the identity.
	g1 := new(bls12381.G1Affine).ScalarMultiplication(&a, target)
	g1 = g1.Neg(g1)
	g2 := new(bls12381.G2Affine).Set(&b)
	g1s = append(g1s, g1)
	g2s = append(g2s, g2)
	return g1s, g2s
}
