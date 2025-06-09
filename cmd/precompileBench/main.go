package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/MariusVanDerWijden/FuzzyVM/generator"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/holiman/goevmlab/fuzzing"
)

func main() {
	worst := time.Duration(0)
	rnd := make([]byte, 10000)
	start := time.Now()
	for i := 0; ; i++ {
		rand.Read(rnd)
		//code := makeSnippet(filler.NewFiller(rnd))
		code := makeEcrecover(filler.NewFiller(rnd))
		//code := makeDataCopy(filler.NewFiller(rnd))
		// code := makeRipeMD(filler.NewFiller(rnd))
		// := makeBlake2f(filler.NewFiller(rnd))
		//code := createBN256Pairing(filler.NewFiller(rnd))
		//code := createBN256Add(filler.NewFiller(rnd))
		// code := createRandomModexp(filler.NewFiller(rnd))
		d := timeGeneration(code)
		if d > worst {
			worst = d
			fmt.Printf("%.2fm: found new worst case, iteration %v: %v \n", time.Since(start).Minutes(), i, d)
			if true || worst > 300*time.Millisecond {
				writeOutTest(code, i)
			}
		}
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
