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

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/holiman/uint256"
)

// p256VerifyAddr is the P256VERIFY / secp256r1 precompile (EIP-7951 / RIP-7212),
// active from Osaka onwards. The generator must target the Osaka fork for this
// to be reachable.
var p256VerifyAddr = common.HexToAddress("0x0100")

type p256Caller struct{}

func (*p256Caller) call(p *program.Program, f *filler.Filler) error {
	var input []byte
	if f.Bool() {
		if in, err := validP256Input(f); err == nil {
			input = in
		} else {
			input = f.ByteSlice(160)
		}
	} else {
		// Invalid input: 160 random bytes exercise the verification-failure path.
		input = f.ByteSlice(160)
	}

	c := CallObj{
		Gas:       uint256.MustFromBig(f.GasInt()),
		Address:   p256VerifyAddr,
		InOffset:  0,
		InSize:    160,
		OutOffset: 0,
		OutSize:   32,
		Value:     f.BigInt32(),
	}
	p.Mstore(input, 0)
	CallRandomizer(p, f, c)
	return nil
}

// validP256Input builds a 160-byte input that verifies:
// hash(32) || r(32) || s(32) || pubX(32) || pubY(32), each big-endian and
// left-padded to 32 bytes.
func validP256Input(f *filler.Filler) ([]byte, error) {
	sk, err := ecdsa.GenerateKey(elliptic.P256(), f)
	if err != nil {
		return nil, err
	}
	hash := f.ByteSlice(32)
	r, s, err := ecdsa.Sign(f, sk, hash)
	if err != nil {
		return nil, err
	}
	input := make([]byte, 160)
	copy(input[0:32], hash)
	copy(input[32:64], leftPad32(r.Bytes()))
	copy(input[64:96], leftPad32(s.Bytes()))
	// sk.X/sk.Y are the deprecated raw-coordinate accessors, but we only read
	// them to build the precompile's fixed-width big-endian pubkey fields, which
	// is exactly what the precompile decodes with big.Int.SetBytes.
	copy(input[96:128], leftPad32(sk.X.Bytes()))
	copy(input[128:160], leftPad32(sk.Y.Bytes()))
	return input, nil
}

// leftPad32 left-pads b to 32 bytes (big-endian), matching the precompile's
// big.Int.SetBytes decoding of each field.
func leftPad32(b []byte) []byte {
	if len(b) >= 32 {
		return b[len(b)-32:]
	}
	out := make([]byte, 32)
	copy(out[32-len(b):], b)
	return out
}
