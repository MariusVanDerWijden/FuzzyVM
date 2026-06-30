package main

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestCreateStateTest(t *testing.T) {
	p := program.New()
	p.Jumpdest()
	p.Op(vm.GAS)
	p.Op(vm.EXTCODECOPY)
	p.Op(vm.JUMP)

	code := p.Bytes()
	writeOutTest(code, 0)
}

func TestRepro(t *testing.T) {
	input := []byte("\xef\xe4\xdde\xa4s")
	code := makeEcrecover(filler.NewFiller(input))
	//code := makeDataCopy(filler.NewFiller(rnd))
	// code := makeRipeMD(filler.NewFiller(rnd))
	// := makeBlake2f(filler.NewFiller(rnd))
	//code := createBN256Pairing(filler.NewFiller(rnd))
	//code := createBN256Add(filler.NewFiller(rnd))
	// code := createRandomModexp(filler.NewFiller(rnd))
	timeGeneration(code)
	writeOutTest(code, 0xffffffff)
}

func FuzzStateTest(f *testing.F) {
	var worst atomic.Uint64
	worst.Store(uint64(time.Duration(0)))
	start := time.Now()
	f.Fuzz(func(t *testing.T, input []byte) {
		//code := makeSnippet(filler.NewFiller(rnd))
		code := makeEcrecover(filler.NewFiller(input))
		//code := makeDataCopy(filler.NewFiller(rnd))
		// code := makeRipeMD(filler.NewFiller(rnd))
		// := makeBlake2f(filler.NewFiller(rnd))
		//code := createBN256Pairing(filler.NewFiller(rnd))
		//code := createBN256Add(filler.NewFiller(rnd))
		// code := createRandomModexp(filler.NewFiller(rnd))
		d := timeGeneration(code)
		for w := time.Duration(worst.Load()); d > w; {
			if !worst.CompareAndSwap(uint64(w), uint64(d)) {
				fmt.Printf("%.2fm: found new worst case: %v \n", time.Since(start).Minutes(), d)
				if w > 300*time.Millisecond {
					writeOutTest(code, int(crypto.Keccak256(code)[0]))
				}
			}
			break
		}
	})
}
