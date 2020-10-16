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

import "testing"

func TestExecute(t *testing.T) {
	err := Execute("../out", "../crashes")
	if err != nil {
		t.Fail()
	}
}

func TestExecuteFullTest(t *testing.T) {
	file := "FuzzyVM-23406873-1687917399.json"
	err := ExecuteFullTest("../out", "../crashes", file, true)
	if err != nil {
		t.Error(err)
	}
}
