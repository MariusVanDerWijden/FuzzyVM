package precompiles

import (
	"bytes"
	"crypto/elliptic"
	"testing"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/ethereum/go-ethereum/crypto"
)

// keySeed is a well-mixed seed; a repetitive one would hide a cursor desync,
// since reading from the wrong offset would give the same bytes anyway.
var keySeed = []byte{
	0x9e, 0x3f, 0x01, 0xb7, 0x5c, 0xa8, 0x2d, 0x44, 0xf1, 0x06, 0x93, 0xce,
	0x7a, 0x28, 0xbd, 0x50, 0x11, 0xe6, 0x8f, 0x3c, 0x62, 0xd9, 0x04, 0xab,
	0x77, 0x1a, 0xc5, 0x38, 0x84, 0x2f, 0xf0, 0x6b, 0x19, 0xdd, 0x53, 0xa1,
	0x0c, 0xb2, 0x7e, 0x45, 0xe9, 0x30, 0x8a, 0x66, 0xcf, 0x21, 0x5b, 0x98,
}

// TestFillerKeyDeterministic checks that key derivation is a pure function of
// the filler and consumes a fixed number of bytes. Both halves matter: an
// ecdsa.GenerateKey-style variable-length read would leave the cursor at a
// different offset and corrupt every later draw in the generation, which is what
// the trailing-bytes comparison catches.
func TestFillerKeyDeterministic(t *testing.T) {
	for _, curve := range []elliptic.Curve{elliptic.P256(), crypto.S256()} {
		var (
			wantKey   = fillerKey(filler.NewFiller(keySeed), curve)
			wantAfter []byte
		)
		for i := 0; i < 16; i++ {
			f := filler.NewFiller(keySeed)
			sk := fillerKey(f, curve)
			if sk.D.Cmp(wantKey.D) != 0 || sk.X.Cmp(wantKey.X) != 0 || sk.Y.Cmp(wantKey.Y) != 0 {
				t.Fatalf("%s: key %d differs from the first key derived from the same seed", curve.Params().Name, i)
			}
			if !curve.IsOnCurve(sk.X, sk.Y) {
				t.Fatalf("%s: public key is not on the curve", curve.Params().Name)
			}
			if sk.D.Sign() <= 0 || sk.D.Cmp(curve.Params().N) >= 0 {
				t.Fatalf("%s: private scalar %v out of range", curve.Params().Name, sk.D)
			}
			after := f.ByteSlice(32)
			if wantAfter == nil {
				wantAfter = after
			} else if !bytes.Equal(after, wantAfter) {
				t.Fatalf("%s: filler cursor moved differently on run %d: %x != %x", curve.Params().Name, i, after, wantAfter)
			}
		}
	}
}

// TestValidP256InputDeterministic checks that the P256VERIFY input, signature
// included, is reproducible from the seed. Deriving the key deterministically is
// not enough on its own: a randomized ECDSA nonce would put different r and s
// into the program on every run.
func TestValidP256InputDeterministic(t *testing.T) {
	var want, wantAfter []byte
	for i := 0; i < 16; i++ {
		f := filler.NewFiller(keySeed)
		input, err := validP256Input(f)
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		after := f.ByteSlice(32)
		if want == nil {
			want, wantAfter = input, after
			continue
		}
		if !bytes.Equal(input, want) {
			t.Fatalf("run %d produced a different input:\n got %x\nwant %x", i, input, want)
		}
		if !bytes.Equal(after, wantAfter) {
			t.Fatalf("run %d left the filler cursor elsewhere: %x != %x", i, after, wantAfter)
		}
	}
}

// TestEcrecoverInputDeterministic does the same for the ecrecover caller, which
// builds its input from a secp256k1 key. crypto.Sign is already deterministic,
// so only the key derivation was at fault here.
func TestEcrecoverInputDeterministic(t *testing.T) {
	var want, wantAfter []byte
	for i := 0; i < 16; i++ {
		var (
			f = filler.NewFiller(keySeed)
			p = program.New()
		)
		if err := (&ecdsaCaller{}).call(p, f); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		var (
			code  = p.Bytes()
			after = f.ByteSlice(32)
		)
		if want == nil {
			want, wantAfter = code, after
			continue
		}
		if !bytes.Equal(code, want) {
			t.Fatalf("run %d produced different code (%d vs %d bytes)", i, len(code), len(want))
		}
		if !bytes.Equal(after, wantAfter) {
			t.Fatalf("run %d left the filler cursor elsewhere: %x != %x", i, after, wantAfter)
		}
	}
}

// TestEcrecoverInputValid checks that the signature we build actually recovers,
// i.e. that fillerKey produces a usable secp256k1 key and the 128-byte layout
// still matches what the precompile decodes.
func TestEcrecoverInputValid(t *testing.T) {
	var (
		f    = filler.NewFiller(keySeed)
		sk   = fillerKey(f, crypto.S256())
		hash = f.ByteSlice(32)
	)
	sig, err := crypto.Sign(hash, sk)
	if err != nil {
		t.Fatalf("signing: %v", err)
	}
	input := make([]byte, 128)
	copy(input[0:32], hash)
	input[63] = sig[64] + 27
	copy(input[64:96], sig[0:32])
	copy(input[96:128], sig[32:64])

	prec := vm.PrecompiledContractsHomestead[common.BytesToAddress([]byte{1})]
	if prec == nil {
		t.Fatal("ecrecover (0x01) not present")
	}
	out, err := prec.Run(input)
	if err != nil {
		t.Fatalf("precompile errored: %v", err)
	}
	want := crypto.PubkeyToAddress(sk.PublicKey)
	if !bytes.Equal(out[12:], want.Bytes()) {
		t.Fatalf("recovered %x, want %x", out[12:], want)
	}
}
