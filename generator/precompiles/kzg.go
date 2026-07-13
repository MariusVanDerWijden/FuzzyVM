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
	"crypto/sha256"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// kzgPointEvaluationAddr is the KZG point-evaluation precompile (EIP-4844),
// active from Cancun onwards.
var kzgPointEvaluationAddr = common.HexToAddress("0x0a")

type kzgCaller struct{}

func (*kzgCaller) call(p *program.Program, f *filler.Filler) error {
	var input []byte
	// Building a valid input runs a real KZG proof (~20ms), which is far more
	// expensive than any other generator step and gets re-run during
	// minimization. Take the valid path only ~1/8 of the time so it still
	// exercises the proof-verification math without dominating throughput.
	if f.Byte() < 32 {
		// Valid input: a real commitment/proof over a random blob. If the KZG
		// setup can't be built (e.g. bad randomness), fall back to random bytes
		// rather than failing the whole generation.
		if in, err := validKZGInput(f); err == nil {
			input = in
		} else {
			input = f.ByteSlice(192)
		}
	} else {
		// Invalid input: 192 random bytes, exercising the error paths.
		input = f.ByteSlice(192)
	}

	c := CallObj{
		Gas:       uint256.MustFromBig(f.GasInt()),
		Address:   kzgPointEvaluationAddr,
		InOffset:  0,
		InSize:    192,
		OutOffset: 0,
		OutSize:   64,
		Value:     f.BigInt32(),
	}
	p.Mstore(input, 0)
	CallRandomizer(p, f, c)
	return nil
}

// WarmupKZG pays the one-time KZG trusted-setup initialization cost (~1.8s in
// go-ethereum's kzg4844) up front, so it doesn't stall the first fuzzing input
// that happens to build a valid KZG proof. Safe to call more than once.
func WarmupKZG() {
	// A short, fixed seed is enough to force the lazy setup; ignore the result.
	_, _ = validKZGInput(filler.NewFiller([]byte{1, 2, 3, 4, 5, 6, 7, 8}))
}

// validKZGInput builds the 192-byte point-evaluation input:
// versionedHash(32) || point(32) || claim(32) || commitment(48) || proof(48).
func validKZGInput(f *filler.Filler) ([]byte, error) {
	// A blob is 128KB. Draw only a small seed from the shared filler cursor and
	// expand it deterministically to fill the blob, rather than pulling 128KB
	// through the cursor (which wraps thousands of times on a small input and,
	// worse, desyncs every subsequent read in the generation).
	seed := f.ByteSlice(64)
	random := expandKZGSeed(seed, 131072)
	blob := encodeKZGBlob(random)
	commitment, err := kzg4844.BlobToCommitment(&blob)
	if err != nil {
		return nil, err
	}
	var point kzg4844.Point
	copy(point[:], random)
	point[0] = 0 // point must be < the field modulus
	proof, claim, err := kzg4844.ComputeProof(&blob, point)
	if err != nil {
		return nil, err
	}
	versionedHash := kzgToVersionedHash(&commitment)

	input := make([]byte, 192)
	copy(input[0:32], versionedHash[:])
	copy(input[32:64], point[:])
	copy(input[64:96], claim[:])
	copy(input[96:144], commitment[:])
	copy(input[144:192], proof[:])
	return input, nil
}

// expandKZGSeed deterministically expands a short seed to n bytes by chaining
// SHA-256 (a counter-mode PRG). This gives a well-mixed, non-repetitive blob
// from a bounded amount of filler data, so mutating a filler byte still changes
// the blob but doesn't cost 128KB of cursor advance.
func expandKZGSeed(seed []byte, n int) []byte {
	out := make([]byte, 0, n)
	block := sha256.Sum256(seed)
	for len(out) < n {
		out = append(out, block[:]...)
		block = sha256.Sum256(block[:])
	}
	return out[:n]
}

// encodeKZGBlob packs data into a single blob, leaving the high byte of each
// 32-byte field element zero so every element stays below the field modulus.
func encodeKZGBlob(data []byte) kzg4844.Blob {
	var blob kzg4844.Blob
	fieldIndex := 0
	for i := 0; i < len(data) && fieldIndex < params.BlobTxFieldElementsPerBlob; i += 31 {
		end := i + 31
		if end > len(data) {
			end = len(data)
		}
		copy(blob[fieldIndex*32+1:], data[i:end])
		fieldIndex++
	}
	return blob
}

// kzgToVersionedHash implements kzg_to_versioned_hash from EIP-4844.
func kzgToVersionedHash(commitment *kzg4844.Commitment) common.Hash {
	h := sha256.Sum256(commitment[:])
	h[0] = 0x01 // VERSIONED_HASH_VERSION_KZG
	return h
}
