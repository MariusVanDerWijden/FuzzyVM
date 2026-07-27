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
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
)

// fillerKey derives an ECDSA private key on curve from the filler.
//
// crypto/ecdsa.GenerateKey must never be handed the filler. It begins with
// crypto/internal/randutil.MaybeReadByte, which consumes zero or one byte from
// the reader depending on a racing select, deliberately so that callers cannot
// depend on deterministic key generation.
func fillerKey(f *filler.Filler, curve elliptic.Curve) *ecdsa.PrivateKey {
	var (
		params = curve.Params()
		one    = big.NewInt(1)
		max    = new(big.Int).Sub(params.N, one)
	)
	// Draw eight bytes more than the order is wide, reduce modulo N-1 and add
	// one: the result is in [1, N-1] with negligible modular bias. This is the
	// construction crypto/ecdsa uses in generateLegacy, minus the nondeterminism.
	d := new(big.Int).SetBytes(f.ByteSlice(params.BitSize/8 + 8))
	d.Mod(d, max)
	d.Add(d, one)

	sk := &ecdsa.PrivateKey{D: d}
	sk.Curve = curve
	// ScalarBaseMult is deprecated in favour of crypto/ecdh, which cannot
	// represent a secp256k1 key and does not expose the raw coordinates the
	// precompile inputs are built from.
	sk.X, sk.Y = curve.ScalarBaseMult(d.Bytes())
	return sk
}
