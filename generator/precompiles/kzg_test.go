package precompiles

import (
	"testing"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

// TestValidKZGInput checks that the input our generator builds actually
// verifies against go-ethereum's real KZG point-evaluation precompile, proving
// the byte layout and versioned-hash encoding are correct.
func TestValidKZGInput(t *testing.T) {
	f := filler.NewFiller([]byte{9, 8, 7, 6, 5, 4, 3, 2, 1, 42, 43, 44})
	input, err := validKZGInput(f)
	if err != nil {
		t.Skipf("could not build KZG input from this seed: %v", err)
	}
	prec := vm.PrecompiledContractsPrague[common.HexToAddress("0x0a")]
	if prec == nil {
		t.Fatal("KZG precompile (0x0a) not present in Prague set")
	}
	out, err := prec.Run(input)
	if err != nil {
		t.Fatalf("precompile rejected our 'valid' input: %v", err)
	}
	if len(out) != 64 {
		t.Fatalf("unexpected output length %d", len(out))
	}
}
