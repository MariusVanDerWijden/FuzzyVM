package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/MariusVanDerWijden/FuzzyVM/generator"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/holiman/goevmlab/fuzzing"
	"golang.org/x/sync/errgroup"
)

type test func(*filler.Filler) []byte

var allTests = []test{
	makeSnippet,
	makeEcrecover,
	makeDataCopy,
	makeRipeMD,
	makeBlake2f,
	createBN256Add,
	createBN256Pairing,
	createRandomModexp,
	createBN256Mul,
	makeBLSMulExpG1,
	makeBLSMulExpG2,
	makeBLSAdd,
	makeBLSAddG2,
	makeBLSMapG1,
	makeBLSMapG2,
	makeBLSPairing,
}

func main() {
	test := len(allTests) - 1
	writeTest := true
	findWorstCases(allTests[test], writeTest)
}

func findWorstCases(generator test, writeTest bool) {
	var worst atomic.Uint64
	var count atomic.Uint64
	worst.Store(uint64(time.Duration(0)))
	start := time.Now()
	for {
		var group errgroup.Group
		group.SetLimit(1)
		for range 10000 {
			group.Go(func() error {
				rnd := make([]byte, 10000)
				rand.Read(rnd)
				code := generator(filler.NewFiller(rnd))
				d := timeGeneration(code)
				for w := time.Duration(worst.Load()); d > w; {
					if !worst.CompareAndSwap(uint64(w), uint64(d)) {
						cnt := count.Add(1)
						fmt.Printf("%.2fm: found new worst case, cnt %v: %v \n", time.Since(start).Minutes(), cnt, d)
						if writeTest || d > 1*time.Second {
							writeOutTest(code, int(cnt))
							if writeTest {
								panic("shutting down")
							}
						}
						return nil
					}
				}
				return nil
			})
		}
		group.Wait()
	}
}

func makeSnippet(f *filler.Filler) []byte {
	p := program.New()
	_, dest := p.Jumpdest()
	p.Append(f.ByteSlice(int(f.Byte())))
	p.Jump(dest)
	return p.Bytes()
}

func timeGeneration(code []byte) time.Duration {
	f := filler.NewFiller([]byte("\x5a\x5a\x5a\x5a\x5a\x5a\x5a"))
	testMaker := generator.CreateGstMaker(f, code)
	start := time.Now()
	if err := testMaker.Fill(nil); err != nil {
		panic(err)
	}
	return time.Since(start)
}

func writeOutTest(code []byte, iteration int) {
	f := filler.NewFiller([]byte("\x5a\x5a\x5a\x5a\x5a\x5a\x5a"))
	testMaker := generator.CreateGstMaker(f, code)
	if err := testMaker.Fill(nil); err != nil {
		panic(err)
	}
	storeTest(testMaker.ToGeneralStateTest("test"), fmt.Sprintf("statetest-%d.json", iteration))
}

// storeTest saves a testcase to disk
// returns true if a duplicate test was found
func storeTest(test *fuzzing.GeneralStateTest, path string) bool {
	// check if the test is already on disk
	if _, err := os.Stat(path); err == nil {
		return true
	} else if !os.IsNotExist(err) {
		panic(err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0755)
	if err != nil {
		panic(fmt.Sprintf("Could not open test file %q: %v", path, err))
	}
	defer f.Close()
	// Write to file
	encoder := json.NewEncoder(f)
	if err = encoder.Encode(test); err != nil {
		panic(fmt.Sprintf("Could not encode state test %q: %v", path, err))
	}
	return false
}
