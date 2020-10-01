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
	"io/ioutil"
	"os"
	"strings"

	"github.com/holiman/goevmlab/evms"
)

var (
	vms = []evms.Evm{
		evms.NewGethEVM("/home/matematik/go/src/github.com/ethereum/go-ethereum/build/bin/evm"),
		evms.NewParityVM("/home/matematik/ethereum/openethereum/target/release/openethereum-evm"),
		evms.NewNethermindVM("/home/matematik/ethereum/nethermind/nethtest"),
		//evms.NewBesuVM("/home/matematik/ethereum/besu/ethereum/evmtool/build/install/evmtool/bin/evm"),
	}
)

func Execute(dirName string) error {
	infos, err := ioutil.ReadDir(dirName)
	if err != nil {
		return err
	}
	for _, info := range infos {
		// All generated tests end in .json
		if strings.HasSuffix(info.Name(), ".json") {
			var (
				testFile  = info.Name()
				testName  = strings.TrimRight(testFile, ".json")
				traceName = fmt.Sprintf("%v-trace.jsonl")
			)
			outputs := executeTest(testFile)
			if !verify(traceName, outputs) {
				if err := dump(testName, vms, outputs); err != nil {
					return err
				}
			}
		}
	}
	// All tests sucessfully executed
	return nil
}

// executeTest executes a state test
func executeTest(testName string) []bytes.Buffer {
	buf := make([]bytes.Buffer, len(vms))
	for i, vm := range vms {
		vm.RunStateTest(testName, &buf[i], false)
	}
	return buf
}

// verify checks if the traces match the default trace.
func verify(traceName string, outputs []bytes.Buffer) bool {
	ref, err := ioutil.ReadFile(traceName)
	if err != nil {
		panic(err)
	}
	for _, out := range outputs {
		if !bytes.Equal(ref, out.Bytes()) {
			return false
		}
	}
	return true
}

// dump writes outputs to a file in case of a verification problem
func dump(filename string, vms []evms.Evm, outputs []bytes.Buffer) error {
	for i, out := range outputs {
		filename := fmt.Sprintf("%v-%v-trace.jsonl", filename, vms[i].Name())
		if err := ioutil.WriteFile(filename, out.Bytes(), os.ModeAppend); err != nil {
			return err
		}
	}
}
