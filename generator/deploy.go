// Copyright 2021 Marius van der Wijden
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
	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
)

// deployInitCode wraps runtime bytecode in constructor (init) code that deploys
// it.
func deployInitCode(runtime []byte) []byte {
	return program.New().ReturnViaCodeCopy(runtime).Bytes()
}

// makeDeployInit builds constructor code for `runtime` — usually a valid deployer,
// but occasionally a deliberately-broken one so the CREATE *failure* branches are
// exercised too.
func (env Environment) makeDeployInit(runtime []byte) []byte {
	switch env.f.Byte() % 8 {
	case 0:
		return program.New().Push(0).Push(0).Op(vm.REVERT).Bytes()
	case 1:
		return deployInitCode(append([]byte{0xEF}, runtime...))
	default:
		return deployInitCode(runtime)
	}
}

// DeployAndCall deploys `runtime` as a real contract and then calls into it with
// callOp.
func (env Environment) DeployAndCall(runtime []byte, isCreate2 bool, callOp vm.OpCode) {
	env.CreateAndCall(env.makeDeployInit(runtime), isCreate2, callOp)
}

// writeOp returns short runtime bytecode that performs a single state-modifying
// operation with valid operands.
func writeOp(f *filler.Filler) []byte {
	p := program.New()
	switch f.Byte() % 6 {
	case 0:
		p.Sstore(f.BigInt256(), f.BigInt256())
	case 1:
		p.Tstore(f.BigInt256(), f.BigInt256())
	case 2:
		n := vm.OpCode(f.Byte() % 5) // LOG0..LOG4
		for i := 0; i < int(n); i++ {
			p.Push(f.BigInt256()) // topic (value irrelevant to the guard)
		}
		p.Push(0).Push(0) // mSize, then mStart on top
		p.Op(vm.LOG0 + n)
	case 3:
		// Only a value-bearing CALL is write-protected in a static frame; nil gas
		// makes the helper emit GAS to forward everything.
		p.Call(nil, f.BigInt256(), 1, 0, 0, 0, 0)
	case 4:
		p.Push(0).Push(0).Push(0).Op(vm.CREATE)
	default:
		p.Selfdestruct(f.BigInt256())
	}
	return p.Bytes()
}
