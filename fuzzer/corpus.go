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
	"math"
	"math/rand"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/MariusVanDerWijden/FuzzyVM/generator"
	"github.com/korovkin/limiter"
)

var cutoff = 10

func CreateNewTest() ([]byte, error) {
	r := make([]byte, 3000000)
	_, err := rand.Read(r)
	if err != nil {
		return []byte{}, err
	}
	return createTest(r)
}

func createTest(data []byte) ([]byte, error) {
	right := len(data)
	left := 0
	b := data[0 : len(data)/2]
	for i := 0; i < int(math.Log(float64(len(data)))+1)+1; i++ {
		f := filler.NewFiller(b)
		generator.GenerateProgram(f)
		if !f.UsedUp() {
			// valid test
			right = len(b)
			mid := left + (right-left)/2
			b = data[0:mid]
		} else {
			left = len(b)
			mid := left + (right-left)/2
			b = data[0:mid]
		}
		if right-left < cutoff {
			return b, nil
		}
	}
	return b, nil
}

// SampleLengthCorpus creates N valid inputs and samples their length.
// It returns the unsorted array of lengths
func SampleLengthCorpus(N int) []int {
	res := make([]int, 0, N)
	resChan := make(chan int, N)
	limit := limiter.NewConcurrencyLimiter(8)
	for i := 0; i < N; i++ {
		fn := func() {
			res, err := CreateNewTest()
			if err != nil {
				fmt.Println("Error")
			}
			resChan <- len(res)
		}
		limit.Execute(fn)
		if i%10 == 0 {
			fmt.Printf("%v: %v\n", i, len(res))
		}
	}
	limit.Wait()
	for i := 0; i < N; i++ {
		res = append(res, <-resChan)
	}
	return res
}
