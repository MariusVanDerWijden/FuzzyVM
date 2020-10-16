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

// Package executor executes state tests and compares results.
package executor

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/holiman/goevmlab/evms"
	"github.com/korovkin/limiter"
	"github.com/pkg/errors"
)

var (
	vms = []evms.Evm{
		evms.NewGethEVM("/home/matematik/ethereum/FuzzyVM/vms/geth-evm"),
		evms.NewParityVM("/home/matematik/ethereum/FuzzyVM/vms/openethereum-evm"),
		evms.NewNethermindVM("/home/matematik/ethereum/FuzzyVM/vms/nethtest"),
		evms.NewBesuVM("/home/matematik/ethereum/besu/ethereum/evmtool/build/install/evmtool/bin/evm"),
		evms.NewTurboGethEVM("/home/matematik/ethereum/FuzzyVM/vms/turbogeth-evm"),
	}
)

// Execute runs all tests in `dirName` and saves crashers in `outDir`
func Execute(dirName, outDir string) error {
	infos, err := ioutil.ReadDir(dirName)
	if err != nil {
		return err
	}
	errChan := make(chan error)
	limit := limiter.NewConcurrencyLimiter(10)
	meter := metrics.GetOrRegisterMeter("ticks", nil)

	for i, info := range infos {
		// All generated tests end in .json
		if strings.HasSuffix(info.Name(), ".json") {
			fmt.Printf("Executing test: %v of %v, %f per minute \n", i/2, len(infos)/2, meter.Rate1())
			meter.Mark(1)
			name := info.Name()
			job := func() {
				if err := ExecuteFullTest(dirName, outDir, name, true); err != nil {
					err := errors.Wrap(err, fmt.Sprintf("in file: %v", name))
					fmt.Println(err)
					//errChan <- err
				}
			}
			limit.Execute(job)
		}
	}
	for {
		select {
		case err := <-errChan:
			fmt.Println(err)
		default:
			// All tests sucessfully executed
			return nil
		}
	}
}

// ExecuteFullTest executes a single test.
func ExecuteFullTest(dirName, outDir, filename string, doPurge bool) error {
	var (
		testFile  = fmt.Sprintf("%v/%v", dirName, filename)
		testName  = strings.TrimRight(filename, ".json")
		traceFile = fmt.Sprintf("%v/%v-trace.jsonl", dirName, testName)
	)
	outputs, err := executeTest(testFile)
	if err != nil {
		return err
	}
	if !verify(traceFile, outputs) {
		fmt.Printf("Test %v failed, dumping\n", testName)
		if err := dump(testName, outDir, vms, outputs); err != nil {
			return err
		}
	} else {
		if doPurge {
			if err := purge(testFile, traceFile); err != nil {
				return err
			}
		} else {
			printOutputs(outputs)
		}
	}
	return nil
}

// executeTest executes a state test
func executeTest(testName string) ([][]byte, error) {
	var buf [][]byte
	var buffer bytes.Buffer
	for _, vm := range vms {
		buffer.Reset()
		if _, err := vm.RunStateTest(testName, &buffer, false); err != nil {
			return nil, err
		}
		buf = append(buf, buffer.Bytes())
	}
	return buf, nil
}

// verify checks if the traces match the default trace.
func verify(traceName string, outputs [][]byte) bool {
	var ioReaders []io.Reader
	for _, out := range outputs {
		ioReaders = append(ioReaders, bytes.NewReader(out))
	}
	// Add the standard trace to the test (currently deactivated)
	/*
		ref, err := ioutil.ReadFile(traceName)
		if err != nil {
			return false
		}
		ioReaders = append(ioReaders, bytes.NewBuffer(ref))
	*/
	return evms.CompareFiles(vms, ioReaders)
}

// dump writes outputs to a file in case of a verification problem
func dump(filename, outdir string, vms []evms.Evm, outputs [][]byte) error {
	for i, out := range outputs {
		filename := fmt.Sprintf("%v/%v-%v-trace.jsonl", outdir, filename, vms[i].Name())
		f, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0755)
		if err != nil {
			return err
		}
		if _, err := f.Write(out); err != nil {
			return err
		}
	}
	return nil
}

// purge deletes a test file and its corresponding trace
func purge(filename, tracename string) error {
	if err := os.Remove(tracename); err != nil {
		return err
	}
	if err := os.Remove(filename); err != nil {
		return err
	}
	return nil
}

// printOutputs prints out the produced traces
func printOutputs(outputs [][]byte) {
	fmt.Println("TRACES:")
	fmt.Println("--------------")
	for i, out := range outputs {
		fmt.Printf("%v: \n", vms[i].Name())
		fmt.Print(string(out))
		fmt.Println("--------------")
	}
}
