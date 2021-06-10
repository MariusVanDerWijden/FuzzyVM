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
	"bytes"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/urfave/cli.v1"

	"github.com/MariusVanDerWijden/FuzzyVM/benchmark"
	"github.com/MariusVanDerWijden/FuzzyVM/executor"
	"github.com/MariusVanDerWijden/FuzzyVM/fuzzer"
	"github.com/ethereum/go-ethereum/common"
)

func initApp() *cli.App {
	app := cli.NewApp()
	app.Name = "FuzzyVM"
	app.Author = "Marius van der Wijden"
	app.Usage = "Generator for Ethereum Virtual Machine tests"
	app.Action = mainLoop
	app.Flags = []cli.Flag{
		genThreadsFlag,
		maxTestsFlag,
		minTestsFlag,
		buildFlag,
		retestFlag,
		execNoGen,
		benchFlag,
		corpusFlag,
		corpusMinimizeFlag,
		configFileFlag,
	}
	return app
}

var app = initApp()

const (
	dirName = "out"
	outDir  = "crashes"
)

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func mainLoop(c *cli.Context) {
	var (
		execThreads = c.GlobalInt(execThreadsFlag.Name)
		configFile  = c.GlobalString(configFileFlag.Name)
	)

	if c.GlobalBool(corpusMinimizeFlag.Name) {
		minimizeCorpus()
		return
	}

	vms, err := getVMsFromConfig(configFile)
	if err != nil {
		panic(err)
	}
	exec := executor.NewExecutor(vms, true)

	ensureDirs(dirName, outDir)

	if c.GlobalBool(buildFlag.Name) {
		if err := startBuilder(); err != nil {
			panic(err)
		}
	} else if c.GlobalString(retestFlag.Name) != "" {
		retest(c, exec)
	} else if c.GlobalBool(execNoGen.Name) {
		if err := exec.Execute(dirName, outDir, execThreads); err != nil {
			panic(err)
		}
	} else if c.GlobalInt(benchFlag.Name) != 0 {
		benchmark.RunFullBench(c.GlobalInt(benchFlag.Name), execThreads)
	} else if c.GlobalInt(corpusFlag.Name) != 0 {
		createCorpus(c.GlobalInt(corpusFlag.Name))
	} else {
		generatorLoop(c, exec)
	}
}

func startBuilder() error {
	cmdName := "go-fuzz-build"
	cmd := exec.Command(cmdName)
	cmd.Dir = "fuzzer"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// We have to disable CGO
	cgo := "CGO_ENABLED=0"
	env := append(os.Environ(), cgo)
	cmd.Env = env
	return cmd.Run()
}

func retest(c *cli.Context, exec *executor.Executor) {
	p := c.GlobalString(retestFlag.Name)
	dir := path.Dir(p)
	test := path.Base(p)
	exec.ExecuteFullTest(dirName, dir, test, false)
}

func generatorLoop(c *cli.Context, exec *executor.Executor) {
	var (
		genThreads  = c.GlobalInt(genThreadsFlag.Name)
		execThreads = c.GlobalInt(execThreadsFlag.Name)
		minTests    = c.GlobalInt(minTestsFlag.Name)
		maxTests    = c.GlobalInt(maxTestsFlag.Name)
		errChan     = make(chan error)
	)
	for {
		fmt.Println("Starting generator")
		cmd := startGenerator(genThreads)
		go func() {
			for {
				// Sleep a bit to ensure some tests have been generated.
				time.Sleep(30 * time.Second)
				fmt.Println("Starting executor")
				if err := exec.Execute(dirName, outDir, execThreads); err != nil {
					errChan <- err
					return
				}
				errChan <- nil
			}
		}()
		go watcher(cmd, errChan, maxTests)

		err := <-errChan
		cmd.Process.Signal(os.Kill)
		if err != nil {
			panic(err)
		}
		infos, err := ioutil.ReadDir(dirName)
		if err != nil {
			panic(err)
		}
		if len(infos) > minTests {
			fmt.Println("Tests exceed minTests after execution")
			return
		}
	}
}

func startGenerator(genThreads int) *exec.Cmd {
	cmdName := "go-fuzz"
	dir := "./fuzzer/fuzzer-fuzz.zip"
	cmd := exec.Command(cmdName, "--bin", dir, "--procs", fmt.Sprint(genThreads))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	return cmd
}

func watcher(cmd *exec.Cmd, errChan chan error, maxTests int) {
	for {
		time.Sleep(time.Second * 5)
		infos, err := ioutil.ReadDir(dirName)
		if err != nil {
			fmt.Printf("Error killing process: %v\n", err)
			cmd.Process.Kill()
			errChan <- errors.Wrapf(err, "can't open the directory %q", dirName)
		}
		if len(infos) > maxTests {
			fmt.Printf("Max tests exceeded, pausing\n")
			cmd.Process.Signal(os.Interrupt)
			return
		}
	}
}

func createCorpus(n int) {
	const dir = "corpus"
	ensureDirs(dir)

	for i := 0; i < n; i++ {
		elem, err := fuzzer.CreateNewCorpusElement()
		if err != nil {
			fmt.Printf("Error while creating corpus: %v\n", err)
		}
		hash := sha1.Sum(elem)
		filename := fmt.Sprintf("%v/%v", dir, common.Bytes2Hex(hash[:]))
		if err := ioutil.WriteFile(filename, elem, 0755); err != nil {
			fmt.Printf("Error while writing corpus element: %v\n", err)
		}
	}
}

func minimizeCorpus() {
	const dir = "corpus"
	ensureDirs(dir)
	infos, err := ioutil.ReadDir(dirName)
	if err != nil {
		panic(err)
	}
	toDelete := make(map[string]struct{})
	for i, info := range infos {
		f, err := ioutil.ReadFile(info.Name())
		if err != nil {
			continue
		}
		for k, info2 := range infos {
			if k == i {
				continue
			}
			h, err := ioutil.ReadFile(info2.Name())
			if err != nil {
				continue
			}
			if bytes.HasPrefix(h, f) {
				toDelete[info2.Name()] = struct{}{}
			}
		}
	}
	for name := range toDelete {
		fmt.Printf("Removing corpus file: %v\n", name)
		os.Remove(name)
	}
}

func ensureDirs(dirs ...string) {
	for _, dir := range dirs {
		_, err := os.Stat(dir)
		if err != nil {
			fmt.Printf("Error while using os.Stat dir %q: %v\n", dir, err)

			if os.IsNotExist(err) {
				if err = os.Mkdir(dir, 0777); err != nil {
					fmt.Printf("Error while making the dir %q: %v\n", dir, err)
					return
				}
			}
		}
	}
}
