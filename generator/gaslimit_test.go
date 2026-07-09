package generator

import (
	"testing"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
)

// TestGasLimitDistribution checks that gasLimit returns the default most of the
// time but also produces smaller, OOG-inducing limits, and never a value that
// the sender balance can't afford.
func TestGasLimitDistribution(t *testing.T) {
	// sender balance / gas price is the affordable ceiling.
	const gasPrice = 0x80
	const maxAffordable = 0x3fffffffffffffff / gasPrice

	var def, low, other int
	for b := 0; b < 256; b++ {
		// Feed a single leading byte to steer the branch, then padding.
		f := filler.NewFiller([]byte{byte(b), 1, 2, 3, 4, 5, 6, 7})
		g := gasLimit(f)
		if g > maxAffordable {
			t.Fatalf("byte %d: gas limit %d exceeds affordable %d", b, g, maxAffordable)
		}
		switch {
		case g == defaultGasLimit:
			def++
		case g < 100_000:
			low++
		default:
			other++
		}
	}
	if def == 0 || low == 0 {
		t.Fatalf("distribution collapsed: default=%d low=%d other=%d", def, low, other)
	}
	t.Logf("gas-limit branches over 256 seeds: default=%d low=%d other=%d", def, low, other)
}
