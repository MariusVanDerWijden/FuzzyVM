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

package generator

import (
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
)

func TestGenerator(t *testing.T) {
	inputEscaped := "\x83"
	input := []byte(inputEscaped)
	filler := filler.NewFiller(input)
	GenerateProgram(filler)
}

func TestRuntime(t *testing.T) {
	testStart := time.Now()
	for i := byte(0); i < 255; i++ {
		start := time.Now()
		fmt.Printf("Testing with val %v \n", i)
		input := []byte{i}
		filler := filler.NewFiller(input)
		GenerateProgram(filler)
		fmt.Printf("Took %v\n", time.Since(start))
	}
	if time.Since(testStart) > 10*time.Second {
		t.Error("Tests took too long to generate")
	}
	t.Fail()
}

func TestRandomGenerator(t *testing.T) {
	input := make([]byte, 100000)
	rand.Read(input)
	filler := filler.NewFiller(input)
	GenerateProgram(filler)
}
