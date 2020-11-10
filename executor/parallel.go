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

package executor

import (
	"bytes"
	"sync"
)

// executeTestParallel executes a tests in parallel.
// It returns a [][]byte
// In the first dimension we have the len(vms)
// In the second dimension we have len(outputs)
func executeTestParallel(test string) ([][]byte, error) {
	type result struct {
		output []byte
		index  int
	}
	var (
		buf     = make([][]byte, len(vms))
		ch      = make(chan result, len(vms))
		errChan = make(chan error)
		wg      sync.WaitGroup
	)
	wg.Add(len(vms))
	for i, vm := range vms {
		go func() {
			var buffer bytes.Buffer
			if _, err := vm.RunStateTest(test, &buffer, false); err != nil {
				errChan <- err
			}
			ch <- result{output: buffer.Bytes(), index: i}
			wg.Done()
		}()
	}
	wg.Wait()
	for range vms {
		select {
		case err := <-errChan:
			return buf, err
		case res := <-ch:
			buf[res.index] = res.output
		}
	}
	return buf, nil
}

// executeTestBatchParallel executes a batch of tests in parallel.
// It returns a [][][]byte
// In the first dimension we have the len(vms)
// In the second dimension we have len(tests)
// In the third dimension we have len(outputs)
func executeTestBatchParallel(tests []string) ([][][]byte, error) {
	type result struct {
		output [][]byte
		index  int
	}
	var (
		buf     = make([][][]byte, len(vms))
		ch      = make(chan result, len(vms))
		errChan = make(chan error)
		wg      sync.WaitGroup
	)
	wg.Add(len(vms))
	for i, vm := range vms {
		go func() {
			b, err := vm.RunStateTestBatch(tests)
			if err != nil {
				errChan <- err
			}
			ch <- result{output: b, index: i}
			wg.Done()
		}()
	}
	wg.Wait()
	for range vms {
		select {
		case err := <-errChan:
			return buf, err
		case res := <-ch:
			buf[res.index] = res.output
		}
	}
	return buf, nil
}
