package main

import (
	"crypto/sha256"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/params"
)

func makeKZG(f *filler.Filler) []byte {
	commitment, claim, proof, point, err := createPrecompileRandParams(f)
	if err != nil {
		panic(err)
	}

	input := precompileParamsToBytes(commitment, claim, proof, point)
	input[0] = 0
	p := program.New()
	p.Mstore(input, 0)
	_, dest := p.Jumpdest()
	p.StaticCall(nil, 0xa, 0, 192, 192, 32)
	p.Op(vm.POP)
	p.Jump(dest)
	return p.Bytes()
}

func precompileParamsToBytes(commitment *kzg4844.Commitment, claim *kzg4844.Claim, proof *kzg4844.Proof, point *kzg4844.Point) []byte {
	bytes := make([]byte, 192)
	versionedHash := kZGToVersionedHash(commitment)
	copy(bytes[0:32], versionedHash[:])
	copy(bytes[32:64], point[:])
	copy(bytes[64:96], claim[:])
	copy(bytes[96:144], commitment[:])
	copy(bytes[144:192], proof[:])
	return bytes
}

func createPrecompileRandParams(f *filler.Filler) (*kzg4844.Commitment, *kzg4844.Claim, *kzg4844.Proof, *kzg4844.Point, error) {
	random := f.ByteSlice(131072)
	blob := encodeBlobs(random)[0]
	commitment, err := kzg4844.BlobToCommitment(&blob)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var point kzg4844.Point
	copy(point[:], random)
	point[0] = 0 // point needs to be < modulus
	proof, claim, err := kzg4844.ComputeProof(&blob, point)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return &commitment, &claim, &proof, &point, nil
}

func encodeBlobs(data []byte) []kzg4844.Blob {
	blobs := []kzg4844.Blob{{}}
	blobIndex := 0
	fieldIndex := -1
	for i := 0; i < len(data); i += 31 {
		fieldIndex++
		if fieldIndex == params.BlobTxFieldElementsPerBlob {
			blobs = append(blobs, kzg4844.Blob{})
			blobIndex++
			fieldIndex = 0
		}
		max := i + 31
		if max > len(data) {
			max = len(data)
		}
		copy(blobs[blobIndex][fieldIndex*32+1:], data[i:max])
	}
	return blobs
}

// kZGToVersionedHash implements kzg_to_versioned_hash from EIP-4844
func kZGToVersionedHash(kzg *kzg4844.Commitment) common.Hash {
	h := sha256.Sum256(kzg[:])
	h[0] = 0x01

	return h
}
