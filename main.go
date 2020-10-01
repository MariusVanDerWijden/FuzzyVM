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

// Package main creates a fuzzer for Ethereum Virtual Machine (evm) implementations.
package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/MariusVanDerWijden/FuzzyVM/executor"
)

var maxTests = 100000
var minTests = 1000

func main() {
	for {
		errChan := make(chan error)
		cmd := startGenerator()
		go startExecutor(errChan)
		go watcher(cmd, errChan)

		err := <-errChan
		if err != nil {
			panic(err)
		}
		infos, err := ioutil.ReadDir("out")
		if err != nil {
			panic(err)
		}
		if len(infos) > minTests {
			fmt.Println("Tests exceed minTests after execution")
			return
		}
	}
}

func startGenerator() *exec.Cmd {
	cmdName := "go-fuzz"
	dir := "./fuzzer/fuzzer-fuzz.zip"
	cmd := exec.Command(cmdName, "--bin", dir)
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	return cmd
}

func startExecutor(errChan chan error) {
	errChan <- executor.Execute("out", "crashes")
}

func watcher(cmd *exec.Cmd, errChan chan error) {
	for {
		time.Sleep(time.Second * 5)
		infos, err := ioutil.ReadDir("out")
		if err != nil {
			fmt.Printf("Error killing process: %v\n", err)
			cmd.Process.Kill()
			errChan <- err
		}
		if len(infos) > maxTests {
			fmt.Printf("Max tests exceeded, pausing")
			cmd.Process.Signal(os.Interrupt)
			return
		}
	}
}
