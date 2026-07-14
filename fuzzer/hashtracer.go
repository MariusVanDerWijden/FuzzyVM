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

// Package fuzzer is the entry point for go-fuzz.

package fuzzer

import "github.com/ethereum/go-ethereum/core/tracing"

// maxTraceSteps bounds how many opcodes a single probe may trace during
// minimization.
const maxTraceSteps = 1_500_000

// hashTracer folds a program's per-opcode execution trace into a rolling 64-bit
// FNV-1a hash. Comparing two runs by hash is what lets minimization re-execute
// the program ~log2(len) times cheaply: the previous approach serialized a full
// JSON structlog and byte-compared megabytes on every probe, and that
// serialization — not the EVM execution — is what made a single minimize step
// slow enough to trip the watchdog.
type hashTracer struct {
	sum      uint64
	steps    int
	overflow bool
}

func newHashTracer() *hashTracer {
	return &hashTracer{sum: 1469598103934665603} // FNV-1a offset basis
}

func (h *hashTracer) fold(x uint64) {
	h.sum = (h.sum ^ x) * 1099511628211 // FNV-1a prime
}

func (h *hashTracer) hooks() *tracing.Hooks {
	return &tracing.Hooks{
		OnOpcode: func(pc uint64, op byte, gas, cost uint64, scope tracing.OpContext, _ []byte, depth int, _ error) {
			if h.overflow {
				return
			}
			if h.steps++; h.steps > maxTraceSteps {
				h.overflow = true
				return
			}
			h.fold(pc)
			h.fold(uint64(op))
			h.fold(gas)
			h.fold(cost)
			h.fold(uint64(depth))
			// Fold the whole stack so a prefix that reaches the same opcodes with
			// different operands still counts as diverged — matching the fidelity
			// of the JSON structlog this replaces (which included the stack).
			for _, s := range scope.StackData() {
				h.fold(s[0])
				h.fold(s[1])
				h.fold(s[2])
				h.fold(s[3])
			}
		},
	}
}
