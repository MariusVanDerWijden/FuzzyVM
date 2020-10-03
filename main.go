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

	"gopkg.in/urfave/cli.v1"

	"github.com/MariusVanDerWijden/FuzzyVM/executor"
)

var (
	GenProcFlag = cli.StringFlag{
		Name:  "gen.procs",
		Usage: "Number of generator processes started",
		Value: "1",
	}
	MaxTestsFlag = cli.IntFlag{
		Name:  "gen.maxtests",
		Usage: "Number of max tests generated",
		Value: 10000,
	}
	MinTestsFlag = cli.IntFlag{
		Name:  "gen.mintests",
		Usage: "Number of max tests that could fail",
		Value: 1000,
	}
	Build = cli.BoolFlag{
		Name:  "build",
		Usage: "If build is set we run go-fuzz-build",
		Value: false,
	}
)

func initApp() *cli.App {
	app := cli.NewApp()
	app.Name = "FuzzyVM"
	app.Author = "Marius van der Wijden"
	app.Usage = "Generator for Ethereum Virtual Machine tests"
	app.Action = mainLoop
	return app
}

var app = initApp()

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func mainLoop(c *cli.Context) {
	if c.GlobalBool(Build.Name) {
		if err := startBuilder(); err != nil {
			panic(err)
		}
	} else {
		generatorLoop(c)
	}
}

func generatorLoop(c *cli.Context) {
	var (
		genProc  = c.GlobalString(GenProcFlag.Name)
		minTests = c.GlobalInt(MinTestsFlag.Name)
		maxTests = c.GlobalInt(MaxTestsFlag.Name)
	)
	for {
		fmt.Println("Starting generator")
		errChan := make(chan error)
		cmd := startGenerator(genProc)
		go startExecutor(errChan)
		go watcher(cmd, errChan, maxTests)

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

func startBuilder() error {
	cmdName := "go-fuzz-build"
	cmd := exec.Command(cmdName)
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	return cmd.Wait()
}

func startGenerator(genThreads string) *exec.Cmd {
	cmdName := "go-fuzz"
	dir := "./fuzzer/fuzzer-fuzz.zip"
	cmd := exec.Command(cmdName, "--bin", dir, "--procs", genThreads)
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	return cmd
}

func startExecutor(errChan chan error) {
	errChan <- executor.Execute("out", "crashes")
}

func watcher(cmd *exec.Cmd, errChan chan error, maxTests int) {
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
