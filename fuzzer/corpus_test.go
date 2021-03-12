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
	"sort"
	"testing"

	"github.com/korovkin/limiter"
)

func TestCreateCorpus(t *testing.T) {
	res, err := CreateNewCorpusElement()
	if err != nil {
		t.Error(err)
	}
	t.Error(len(res))
}

func TestCreateSpecific(t *testing.T) {
	b := make([]byte, 200)
	res, err := createTest(b)
	if err != nil {
		t.Error(err)
	}
	t.Error(len(res))
}

func TestCreateMaxTest(t *testing.T) {
	max := 0
	i := 0
	limit := limiter.NewConcurrencyLimiter(8)
	for {
		fn := func() {
			res, err := CreateNewCorpusElement()
			if err != nil {
				t.Error(err)
			}
			if len(res) > max {
				max = len(res)
			}
			i++
			fmt.Printf("%v: %v \t %v \n", i, len(res), max)
		}
		limit.Execute(fn)
	}
}

func TestSample(t *testing.T) {
	results := SampleLengthCorpus(10000)
	sort.Ints(results)
	/*
		fmt.Printf("%v", results)
		groups := make(map[int]int)
		for _, res := range results {
			groups[res]++
		}
		for k, r := range groups {
			//fmt.Printf("%v : %v\n", k, r)
		}*/
	subgroups := make(map[int]int)
	for _, res := range results {
		group := res / 500
		subgroups[group*500]++
	}
	for k, r := range subgroups {
		fmt.Printf("%v : %v\n", k, r)
	}
	panic("asdf")
}
