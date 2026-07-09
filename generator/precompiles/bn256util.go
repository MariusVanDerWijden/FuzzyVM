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

	bn254 "github.com/consensys/gnark-crypto/ecc/bn254"
)

// go-ethereum's crypto/bn256 switched to a gnark backend that dropped
// ScalarBaseMult, so we compute k*G directly with gnark-crypto and encode the
// result in the raw EVM point format the bn256 precompiles expect (big-endian
// coordinate pairs, no compression flags — which is why gnark's own Marshal is
// unsuitable here).

// bn256EncodeG1 returns the EVM encoding (64 bytes: x||y) of a bn254 G1 point.
func bn256EncodeG1(p *bn254.G1Affine) []byte {
	out := make([]byte, 64)
	xb := p.X.Bytes()
	yb := p.Y.Bytes()
	copy(out[0:32], xb[:])
	copy(out[32:64], yb[:])
	return out
}

// bn256EncodeG2 returns the EVM encoding (128 bytes: x.A1||x.A0||y.A1||y.A0) of
// a bn254 G2 point.
func bn256EncodeG2(p *bn254.G2Affine) []byte {
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

// bn256G1ScalarBaseMult returns the EVM encoding (64 bytes: x||y) of k*G1.
func bn256G1ScalarBaseMult(k *big.Int) []byte {
	var p bn254.G1Affine
	p.ScalarMultiplicationBase(k)
	return bn256EncodeG1(&p)
}
