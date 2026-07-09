package precompiles

import (
	"testing"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

func blsPrecompile(t *testing.T, addr string) vm.PrecompiledContract {
	t.Helper()
	prec := vm.PrecompiledContractsPrague[common.HexToAddress(addr)]
	if prec == nil {
		t.Fatalf("BLS precompile %s not present in Prague set", addr)
	}
	return prec
}

// TestBLSEncodings checks that the points/elements our generator builds are
// accepted by go-ethereum's real EIP-2537 precompiles, i.e. that our EVM byte
// encoding matches what the precompiles decode.
func TestBLSEncodings(t *testing.T) {
	f := filler.NewFiller([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	// G1Add: two G1 points -> 128-byte output.
	in := append(encodeBLSG1(randG1(f)), encodeBLSG1(randG1(f))...)
	if out, err := blsPrecompile(t, "0x0b").Run(in); err != nil || len(out) != blsG1Size {
		t.Fatalf("G1Add rejected valid input: err=%v len=%d", err, len(out))
	}

	// G2Add: two G2 points -> 256-byte output.
	in = append(encodeBLSG2(randG2(f)), encodeBLSG2(randG2(f))...)
	if out, err := blsPrecompile(t, "0x0d").Run(in); err != nil || len(out) != blsG2Size {
		t.Fatalf("G2Add rejected valid input: err=%v len=%d", err, len(out))
	}

	// MapFpToG1: one 64-byte field element.
	if _, err := blsPrecompile(t, "0x10").Run(encodeBLSFp(f)); err != nil {
		t.Fatalf("MapG1 rejected valid input: %v", err)
	}

	// MapFp2ToG2: two 64-byte field elements.
	in = append(encodeBLSFp(f), encodeBLSFp(f)...)
	if _, err := blsPrecompile(t, "0x11").Run(in); err != nil {
		t.Fatalf("MapG2 rejected valid input: %v", err)
	}

	// G1MSM: one (point, scalar) pair.
	in = append(encodeBLSG1(randG1(f)), scalar32(f)...)
	if _, err := blsPrecompile(t, "0x0c").Run(in); err != nil {
		t.Fatalf("G1MSM rejected valid input: %v", err)
	}
}

// TestBLSPairingVerifies checks that the pairing set we build actually passes
// the EIP-2537 pairing precompile (result should be the "true" word).
func TestBLSPairingVerifies(t *testing.T) {
	f := filler.NewFiller([]byte{17, 4, 9, 2, 8, 8, 1, 5, 3, 3, 7, 6, 2, 9, 9, 1})
	g1s, g2s := blsPairing(2, f)

	var in []byte
	for i := range g1s {
		in = append(in, encodeBLSG1(g1s[i])...)
		in = append(in, encodeBLSG2(g2s[i])...)
	}
	out, err := blsPrecompile(t, "0x0f").Run(in)
	if err != nil {
		t.Fatalf("pairing precompile rejected input: %v", err)
	}
	// A passing pairing returns 32 bytes ending in 0x01.
	if len(out) != 32 || out[31] != 1 {
		t.Fatalf("pairing did not verify: out=%x", out)
	}
}

// sanity: the generators are non-trivial.
var _ = bls12381.Generators
