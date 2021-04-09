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

package main

import (
	"runtime"

	"gopkg.in/urfave/cli.v1"
)

var (
	genThreadsFlag = cli.IntFlag{
		Name:  "gen.threads",
		Usage: "Number of generator threads started (default = 1)",
		Value: 1,
	}
	execThreadsFlag = cli.IntFlag{
		Name:  "exec.threads",
		Usage: "Number of execution threads started (default = NumCPU()",
		Value: runtime.NumCPU(),
	}
	maxTestsFlag = cli.IntFlag{
		Name:  "gen.maxtests",
		Usage: "Number of max tests generated",
		Value: 5000,
	}
	minTestsFlag = cli.IntFlag{
		Name:  "gen.mintests",
		Usage: "Number of max tests that could fail",
		Value: 1000,
	}
	buildFlag = cli.BoolFlag{
		Name:  "build",
		Usage: "If build is set we run go-fuzz-build",
	}
	execNoGen = cli.BoolFlag{
		Name:  "exec",
		Usage: "If exec is set, we only execute not generate new tests",
	}
	retestFlag = cli.StringFlag{
		Name:  "retest",
		Usage: "Rerun the specified test",
		Value: "",
	}
	benchFlag = cli.IntFlag{
		Name:  "bench",
		Usage: "Number of tests that should be benched",
	}
	corpusFlag = cli.IntFlag{
		Name:  "corpus",
		Usage: "Number of corpus elements that should be created",
	}
)
