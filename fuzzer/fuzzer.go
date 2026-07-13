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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/ethereum/go-ethereum/tests"
	"github.com/holiman/goevmlab/fuzzing"
	"golang.org/x/crypto/sha3"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/MariusVanDerWijden/FuzzyVM/generator"
)

var (
	outputDir   = "out"
	EnvKey      = "FUZZYDIR"
	shouldTrace = false
)

// SetFuzzyVMDir sets the output directory for FuzzyVM
// If the environment variable FUZZYDIR is set, the output directory
// will be set to that, otherwise it will be set to a temp dir (for unit tests)
func SetFuzzyVMDir() {
	if dir, ok := os.LookupEnv(EnvKey); ok {
		outputDir = dir
	} else {
		outputDir = os.TempDir()
	}
}

func FuzzStateless(data []byte) int {
	if len(data) < 32 {
		return -1
	}
	f := filler.NewFiller(data)
	generator.GenerateProgram(f)
	return 0
}

// Fuzz is the entry point for go-fuzz
func Fuzz(data []byte) int {
	// Too little data destroys our performance and makes it hard for the generator
	if len(data) < 32 {
		return -1
	}
	f := filler.NewFiller(data)
	testMaker, _ := generator.GenerateProgram(f)
	// Minimize the test. MinimizeProgram runs a full Fill internally, so it is
	// also our execution check: if it succeeds, the test is fillable.
	minimized, _, err := MinimizeProgram(testMaker)
	switch {
	case err == nil:
		testMaker = minimized
	case errors.Is(err, ErrTraceTooLarge):
		// A loop-until-OOG program: too expensive to minimize, but still a valid
		// (unminimized) test worth keeping. Fall through with the original.
	default:
		// The program can't be filled/executed (e.g. a generated transaction
		// whose intrinsic gas exceeds its gas limit). That's a generator-internal
		// condition, not a client discrepancy — skip it rather than crash the
		// campaign the way an unconditional panic would.
		return 0
	}
	hashed := hash(testMaker.ToGeneralStateTest("hashName"))
	finalName := fmt.Sprintf("FuzzyVM-%v", common.Bytes2Hex(hashed))
	// Optionally re-run the test to write out its trace. Any error here is the
	// same recoverable "not fillable" condition as above, so skip rather than
	// panic.
	if shouldTrace {
		traceFile := setupTrace(finalName)
		defer traceFile.Close()
		if err := testMaker.Fill(traceFile, maxTraceSize); err != nil {
			return 0
		}
	}
	// Save the test
	test := testMaker.ToGeneralStateTest(finalName)
	dup, err := storeTest(test, hashed, finalName)
	if err != nil {
		// A filesystem problem is not a reason to crash the campaign.
		fmt.Printf("skipping test that could not be stored: %v\n", err)
		return 0
	}
	if dup {
		return 0
	}
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

// maxTraceSize caps the trace output buffered in memory during minimization.
// Programs that produce larger traces (e.g. loops that run until out of gas)
// are too expensive to minimize and are rejected with ErrTraceTooLarge.
//
// GstMaker.Fill stops after writing maxTraceSize bytes, so a trace that hit the
// limit comes back at (or just under) maxTraceSize with no error. We can't tell
// such a truncated trace from a genuine one, so treat anything within
// traceSizeMargin of the limit as overflow: a result of 32MB-1k is almost
// certainly a program that would have exceeded 32MB.
const (
	maxTraceSize    = 32 * 1024 * 1024
	traceSizeMargin = 1024
)

var ErrTraceTooLarge = errors.New("trace too large to minimize")

// cappedBuffer buffers writes and flags overflow once it comes within
// traceSizeMargin of maxTraceSize.
type cappedBuffer struct {
	buf      bytes.Buffer
	overflow bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if c.overflow || c.buf.Len()+len(p) > maxTraceSize-traceSizeMargin {
		c.overflow = true
		return len(p), nil
	}
	return c.buf.Write(p)
}

func MinimizeProgram(test *fuzzing.GstMaker) (*fuzzing.GstMaker, []byte, error) {
	original := new(cappedBuffer)
	if err := test.Fill(original, maxTraceSize); err != nil {
		return nil, nil, err
	}
	if original.overflow {
		return nil, nil, ErrTraceTooLarge
	}
	name := ""
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
	orgs := original.buf.Bytes()
	idx := strings.LastIndex(string(orgs), "{")
	if idx <= 0 {
		idx = 0
	} else {
		idx -= 1
	}
	orgs = orgs[0:idx]
	foundLength := sort.Search(len(code), func(i int) bool {
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
		newOutput := new(cappedBuffer)
		cfg := vm.Config{}
		cfg.Tracer = logger.NewJSONLogger(&logger.Config{Limit: maxTraceSize}, newOutput)
		subtest := gethStateTest.Subtests()[0]
		gethStateTest.RunNoVerify(subtest, cfg, false, rawdb.HashScheme)
		if newOutput.overflow {
			// The prefix traces longer than the whole original program, so it
			// cannot match.
			return false
		}
		newB := newOutput.buf.Bytes()
		newIdx := strings.LastIndex(string(newB), "{")
		if newIdx <= 0 {
			newIdx = 0
		} else {
			newIdx -= 1
		}
		newB = newB[0:newIdx]
		//fmt.Printf("%v: %v %v\n", i, len(newB), len(orgs))
		//fmt.Printf(string(newB))
		return bytes.Equal(newB, orgs)
	})
	if foundLength+100 < len(code) {
		// Add some bytes to make it easier to proof differences in execution
		foundLength += 100
	}
	test.SetCode(addr, code[0:foundLength])
	return test, code[0:foundLength], nil
}

// storeTest saves a testcase to disk. It returns (duplicate, err): duplicate is
// true if the test was already present. A filesystem error (disk full,
// permissions, …) is returned rather than panicked, so a transient problem
// mid-campaign is skipped and logged instead of crashing the fuzzer (and being
// misreported by the harness as a discrepancy).
func storeTest(test *fuzzing.GeneralStateTest, hashed []byte, testName string) (bool, error) {
	path := fmt.Sprintf("%v/%02x/%v.json", outputDir, hashed[0], testName)
	// check if the test is already on disk
	if _, err := os.Stat(path); err == nil {
		fmt.Println("Duplicate test found")
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0755)
	if err != nil {
		return false, fmt.Errorf("could not open test file %q: %w", testName, err)
	}
	defer f.Close()
	// Write to file
	encoder := json.NewEncoder(f)
	if err = encoder.Encode(test); err != nil {
		return false, fmt.Errorf("could not encode state test %q: %w", testName, err)
	}
	return false, nil
}

func hash(test *fuzzing.GeneralStateTest) []byte {
	h := sha3.New256()
	encoder := json.NewEncoder(h)
	if err := encoder.Encode(test); err != nil {
		panic(fmt.Sprintf("Could not hash state test: %v", err))
	}
	return h.Sum(nil)
}
