package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/MariusVanDerWijden/FuzzyVM/generator"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/holiman/goevmlab/fuzzing"
	"golang.org/x/sync/errgroup"
)

type test func(*filler.Filler) []byte

var allTests = []test{
	makeSnippet,
	makeEcrecover,
	makeBlake2f,
	createBN256Pairing,
	createRandomModexp,
	makeMstore,
	makeSStore,
	makeKZG,
	createBN256Mul,
	makeMstore2,
	makeSdiv,
	makeSStore2,
	makeR1Recover,
	makeBLSMulExpG1,
	makeBLSMulExpG2,
	makeBLSAddG2,
	makeBLSMapG1,
	makeBLSMapG2,
	makeBLSPairing,
	makeBLSAdd,
	createBN256Add,
	makeDataCopy,
	makeRipeMD,
	makeEcrecover,
}

func main() {
	/*
		test := len(allTests) - 1
		writeTest := true
		findWorstCases(allTests[test], writeTest)
	*/
	//makeCallerTest()
	writeOutAllCalleeAccounts()
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

type contract func(int) []byte

func writeOutAllCalleeAccounts() {

	writeCalleeAccounts("random_128_24k", 128, 24*1024, randomCalleeContract)
	writeCalleeAccounts("random_12000_24k", 12_000, 24*1024, randomCalleeContract)
	writeCalleeAccounts("random_12000_48k", 12_000, 48*1024, randomCalleeContract)
	writeCalleeAccounts("random_12000_96k", 12_000, 96*1024, randomCalleeContract)
	writeCalleeAccounts("random_12000_128k", 12_000, 128*1024, randomCalleeContract)
	writeCalleeAccounts("random_37888_48k", 37*1024, 48*1024, randomCalleeContract)
	writeCalleeAccounts("push2_12000_48k", 12_000, 48*1024, push2CalleeContract)
	writeCalleeAccounts("jumpdest_12000_48k", 12_000, 48*1024, jumpdestCalleeContract)
}

func writeCalleeAccounts(name string, count, size int, codeFn contract) {
	accounts := make(map[common.Address]fuzzing.GenesisAccount, 0)
	for i := range count {
		rawAddr := make([]byte, 20)
		binary.BigEndian.PutUint16(rawAddr, uint16(i))
		acc := common.BytesToAddress(rawAddr)
		accounts[acc] = fuzzing.GenesisAccount{
			Code:    codeFn(size),
			Nonce:   1,
			Balance: common.Big0,
		}
	}
	path := name + ".json"
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0755)
	if err != nil {
		panic(fmt.Sprintf("Could not open test file %q: %v", path, err))
	}
	defer f.Close()
	// Write to file
	encoder := json.NewEncoder(f)
	if err = encoder.Encode(accounts); err != nil {
		panic(fmt.Sprintf("Could not encode state test %q: %v", path, err))
	}
}

// count := 1024 * 37
func callerContract(count int) ([]byte, []common.Address) {
	accounts := []common.Address{}
	p := program.New()
	for range count {
		rnd := make([]byte, 20)
		rand.Read(rnd)
		acc := common.BytesToAddress(rnd)
		accounts = append(accounts, acc)
		p.StaticCall(nil, acc, 0, 0, 0, 0)
		p.Op(vm.POP)
	}
	return p.Bytes(), accounts
}

func callerContract2(count int) ([]byte, []common.Address) {
	accounts := []common.Address{}
	p := program.New()
	for range count {
		rnd := make([]byte, 20)
		rand.Read(rnd)
		acc := common.BytesToAddress(rnd)
		accounts = append(accounts, acc)
		p.Push(acc)
		p.Op(vm.EXTCODESIZE)
		p.Op(vm.POP)
	}
	return p.Bytes(), accounts
}

func randomCalleeContract(size int) []byte {
	p := program.New()
	p.Jump(size - 5)
	rnd := make([]byte, size-128-5)
	rand.Read(rnd)
	p.Append(rnd)
	for range 128 {
		p.Op(vm.JUMPDEST)
	}
	return p.Bytes()
}

func jumpdestCalleeContract(size int) []byte {
	p := program.New()
	p.Jump(size - 5)
	for range size - 128 - 5 {
		p.Op(vm.JUMPDEST)
	}
	for range 128 {
		p.Op(vm.JUMPDEST)
	}
	return p.Bytes()
}

func push2CalleeContract(size int) []byte {
	p := program.New()
	p.Jump(size - 5)
	for range size - 128 - 5 {
		p.Op(vm.PUSH2)
		p.Append([]byte{0xff, 0xff})
	}
	for range 128 {
		p.Op(vm.JUMPDEST)
	}
	return p.Bytes()
}

func makeCallerTest() {
	count := 1024 * 37
	code, accounts := callerContract2(count)
	contractCode := randomCalleeContract(32 * 1024)
	f := filler.NewFiller([]byte("\x5a\x5a\x5a\x5a\x5a\x5a\x5a"))
	testMaker := generator.CreateGstMaker(f, code)
	start := time.Now()
	if err := testMaker.Fill(nil); err != nil {
		panic(err)
	}
	for _, account := range accounts {
		testMaker.AddAccount(account, fuzzing.GenesisAccount{
			Code:    contractCode,
			Nonce:   1,
			Balance: common.Big0,
		})
	}
	fmt.Printf("Filled in %v\n", time.Since(start))
	storeTest(testMaker.ToGeneralStateTest("test"), "statetest-sload.json")
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
