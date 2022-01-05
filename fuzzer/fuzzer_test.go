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

package fuzzer

import (
	"fmt"
	"testing"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/MariusVanDerWijden/FuzzyVM/generator"
)

func TestFuzzer(t *testing.T) {
	data := "asdfasdfasdfasdfasdfasdfasdffasdfasdfasdfasdfasd"
	Fuzz([]byte(data))
	panic("adaf")
}

func TestMinimizeProgram(t *testing.T) {

	// Only local test, should not be run in test pipeline
	data := "asdfadfasdfasdfasdfasdfasdfadsfldlafdsgoinsfandofaijdsf"
	f := filler.NewFiller([]byte(data))
	testMaker, _ := generator.GenerateProgram(f)
	name := randTestName([]byte(data))
	if err := testMaker.Fill(nil); err != nil {
		panic(err)
	}
	// Save the test
	test := testMaker.ToGeneralStateTest(name)
	storeTest(test, name)
	// minimize
	minimized, err := minimizeProgram(testMaker, name)
	if err != nil {
		t.Error(err)
	}
	minTest := minimized.ToGeneralStateTest(name)
	_ = minTest
	fmt.Printf("%v", minTest)
	//panic("adsf")
	storeTest(minTest, name+"_min")

}

func TestStoreTest(t *testing.T) {
	/*
		// Only local test, should not be run in test pipeline
		data := "asdf"
		f := filler.NewFiller([]byte(data))
		testMaker, _ := generator.GenerateProgram(f)
		name := randTestName([]byte(data))
		if err := testMaker.Fill(nil); err != nil {
			panic(err)
		}
		test := testMaker.ToGeneralStateTest(name)
		storeTest(test, name)
		storeTest(test, name+"a")
		t.Fail()
	*/
}
