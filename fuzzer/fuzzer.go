// Copyright 2020 Marius van der Wijden
// This file is part of the fuzzy-vm library.
//
// The fuzzy-vm library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The fuzzy-vm library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the fuzzy-vm library. If not, see <http://www.gnu.org/licenses/>.

// Package fuzzer is the entry point for go-fuzz.
package fuzzer

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/tests"
	"github.com/holiman/goevmlab/fuzzing"
	"golang.org/x/crypto/sha3"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/MariusVanDerWijden/FuzzyVM/generator"
)

var (
	outputDir   = "out"
	shouldTrace = false
)

// Fuzz is the entry point for go-fuzz
func Fuzz(data []byte) int {
	// Too little data destroys our performance and makes it hard for the generator
	if len(data) < 32 {
		return -1
	}
	f := filler.NewFiller(data)
	testMaker, _ := generator.GenerateProgram(f)
	name := randTestName(data)
	// minimize test
	minimized, err := minimizeProgram(testMaker, name)
	if err == nil {
		testMaker = minimized
	}
	// Execute the test and write out the resulting trace
	var traceFile *os.File
	if shouldTrace {
		traceFile = setupTrace(name)
		defer traceFile.Close()
	}
	if err := testMaker.Fill(traceFile); err != nil {
		panic(err)
	}
	// Save the test
	test := testMaker.ToGeneralStateTest(name)
	storeTest(test, name)
	if f.UsedUp() {
		return 0
	}
	return 1
}

func setupTrace(name string) *os.File {
	path := fmt.Sprintf("%v/%v-trace.jsonl", outputDir, name)
	traceFile, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0755)
	if err != nil {
		panic("Could not write out trace file")
	}
	return traceFile
}

func minimizeProgram(test *fuzzing.GstMaker, name string) (*fuzzing.GstMaker, error) {
	original := new(bytes.Buffer)
	if err := test.Fill(original); err != nil {
		return nil, err
	}
	gstPtr := test.ToGeneralStateTest(name)
	gst := (*gstPtr)
	var addr common.Address
	var code []byte
	for ad, acc := range gst[name].Pre {
		if len(acc.Code) > len(code) {
			code = acc.Code
			addr = ad
		}
	}
	orgBytes := original.Bytes()
	foundLength := sort.Search(len(code), func(i int) bool {
		//fmt.Printf("i: %v\n", i)
		// Set the code
		acc := gst[name].Pre[addr]
		acc.Code = code[0:i]
		gst[name].Pre[addr] = acc
		// Run and see if the trace still matches
		var gethStateTest tests.StateTest
		data, err := json.Marshal(gst[name])
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal(data, &gethStateTest); err != nil {
			panic(err)
		}
		newOutput := new(bytes.Buffer)
		cfg := vm.Config{}
		cfg.Debug = true
		cfg.Tracer = vm.NewJSONLogger(&vm.LogConfig{}, newOutput)
		subtest := gethStateTest.Subtests()[0]
		gethStateTest.RunNoVerify(subtest, cfg, false)
		return bytes.Equal(newOutput.Bytes(), orgBytes)
	})
	if foundLength+100 < len(code) {
		// Add some bytes to make it easier to proof differences in execution
		foundLength += 100
	}
	test.SetCode(addr, code[0:foundLength])
	return test, nil
}

// storeTest saves a testcase to disk
func storeTest(test *fuzzing.GeneralStateTest, testName string) {
	hash := hash(test, testName)
	path := fmt.Sprintf("%v/%v-%v.json", outputDir, testName, hash)
	// check if the test is already on disk
	if matches, err := filepath.Glob(fmt.Sprintf("%v/*-%v.json", outputDir, hash)); err != nil {
		panic(err)
	} else if len(matches) != 0 {
		fmt.Printf("Duplicate test found\n")
		return
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0755)
	if err != nil {
		panic(fmt.Sprintf("Could not open test file %q: %v", testName, err))
	}
	defer f.Close()
	// Write to file
	encoder := json.NewEncoder(f)
	if err = encoder.Encode(test); err != nil {
		panic(fmt.Sprintf("Could not encode state test %q: %v", testName, err))
	}
}

func randTestName(data []byte) string {
	var seedData [8]byte
	copy(seedData[:], data)
	seed := int64(binary.BigEndian.Uint64(seedData[:]))
	rand := rand.New(rand.NewSource(time.Now().UnixNano() ^ seed))
	return fmt.Sprintf("FuzzyVM-%v-%v", rand.Int31(), rand.Int31())
}

func hash(test *fuzzing.GeneralStateTest, name string) string {
	h := sha3.New256()
	encoder := json.NewEncoder(h)
	if err := encoder.Encode(test); err != nil {
		panic(fmt.Sprintf("Could not hash state test: %v", err))
	}
	hash := h.Sum(nil)
	return common.Bytes2Hex(hash)
}
