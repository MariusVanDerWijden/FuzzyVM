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

type destination struct {
	pc       uint64
	jumpdest uint64
}

type Jumptable struct {
	minDist uint64
	dests   []destination
}

// NewJumptable creates a new Jumptable with a minimum jump distance.
func NewJumptable(minDist uint64) *Jumptable {
	return &Jumptable{
		minDist: minDist,
		dests:   make([]destination, 0),
	}
}

// Push pushes a new destination on the jumptable.
func (j *Jumptable) Push(pc, dest uint64) {
	j.dests = append(j.dests, destination{pc: pc, jumpdest: dest})
}

// Pop removes a suitable destination from the jumptable.
// If no suitable destination
func (j *Jumptable) Pop(pc uint64) uint64 {
	for i, dest := range j.dests {
		// allow forward jumps
		if pc < dest.jumpdest {
			return j.rem(i).jumpdest
		}
		/*
			// allow jump if enough instructions passed
			if pc-dest.pc > j.minDist {
				return j.rem(i).jumpdest
			}
		*/
	}
	// if no suitable destination found, return pc + 1
	return pc + 1
}

func (j *Jumptable) rem(index int) destination {
	dest := j.dests[index]
	j.dests[index] = j.dests[len(j.dests)-1]
	j.dests = j.dests[:len(j.dests)-1]
	return dest
}
