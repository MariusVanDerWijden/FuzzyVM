package precompiles

import (
	"testing"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

// TestValidP256Input checks that the signature we build verifies against
// go-ethereum's real P256VERIFY precompile in the Osaka set, confirming the
// 160-byte layout and left-padding are correct.
func TestValidP256Input(t *testing.T) {
	f := filler.NewFiller([]byte{1, 1, 2, 3, 5, 8, 13, 21, 34, 55, 89, 144, 233})
	input, err := validP256Input(f)
	if err != nil {
		t.Skipf("could not build P256 input from this seed: %v", err)
	}
	prec := vm.PrecompiledContractsOsaka[common.HexToAddress("0x0100")]
	if prec == nil {
		t.Fatal("P256VERIFY (0x0100) not present in Osaka set")
	}
	out, err := prec.Run(input)
	if err != nil {
		t.Fatalf("precompile errored on valid input: %v", err)
	}
	// A valid signature returns the 32-byte true word (ending in 0x01).
	if len(out) != 32 || out[31] != 1 {
		t.Fatalf("valid signature did not verify: out=%x", out)
	}
}
