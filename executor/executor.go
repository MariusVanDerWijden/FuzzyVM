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
		evms.NewGethEVM("/home/matematik/go/src/github.com/ethereum/go-ethereum/build/bin/evm"),
		evms.NewParityVM("/home/matematik/ethereum/openethereum/target/release/openethereum-evm"),
		evms.NewNethermindVM("/home/matematik/ethereum/nethermind/nethtest"),
		evms.NewBesuVM("/home/matematik/ethereum/besu/ethereum/evmtool/build/install/evmtool/bin/evm"),
	}
)

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
			fmt.Printf("Executing test: %v of %v, %v per minute \n", i/2, len(infos)/2, meter.Rate1())
			job := func() {
				if err := executeFullTest(dirName, outDir, info.Name()); err != nil {
					err := errors.Wrap(err, fmt.Sprintf("in file: %v", info.Name()))
					fmt.Println(err)
					errChan <- err
				}
				meter.Mark(1)
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

func executeFullTest(dirName, outDir, filename string) error {
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
		if err := purge(testFile, traceFile); err != nil {
			return err
		}
	}
	return nil
}

// executeTest executes a state test
func executeTest(testName string) ([]*bytes.Buffer, error) {
	var buf []*bytes.Buffer
	for _, vm := range vms {
		var buffer bytes.Buffer
		if _, err := vm.RunStateTest(testName, &buffer, false); err != nil {
			return nil, err
		}
		buf = append(buf, &buffer)
	}
	return buf, nil
}

// verify checks if the traces match the default trace.
func verify(traceName string, outputs []*bytes.Buffer) bool {
	var ioReaders []io.Reader
	for _, out := range outputs {
		ioReaders = append(ioReaders, out)
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
func dump(filename, outdir string, vms []evms.Evm, outputs []*bytes.Buffer) error {
	for i, out := range outputs {
		filename := fmt.Sprintf("%v/%v-%v-trace.jsonl", outdir, filename, vms[i].Name())
		f, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0755)
		if err != nil {
			return err
		}
		if _, err := f.Write(out.Bytes()); err != nil {
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
