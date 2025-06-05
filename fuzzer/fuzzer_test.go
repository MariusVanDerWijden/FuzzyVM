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
	"os"
	"path/filepath"
	"testing"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/MariusVanDerWijden/FuzzyVM/generator"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/holiman/uint256"
)

func init() {
	SetFuzzyVMDir()
	var directories []string
	for i := 0; i < 256; i++ {
		directories = append(directories, fmt.Sprintf("%v/%v", outputDir, common.Bytes2Hex([]byte{byte(i)})))
	}
	ensureDirs(directories...)
}

func ensureDirs(dirs ...string) {
	for _, dir := range dirs {
		_, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("Creating directory: %v\n", dir)
				if err = os.Mkdir(dir, 0777); err != nil {
					fmt.Printf("Error while making the dir %q: %v\n", dir, err)
					return
				}
			} else {
				fmt.Printf("Error while using os.Stat dir %q: %v\n", dir, err)
			}
		}
	}
}

func readCorpus() []string {
	defaultDir := "./../corpus/"
	entries, err := os.ReadDir(defaultDir)
	if err != nil {
		fmt.Printf("Error reading corpus directory: %v\n", err)
	}
	res := make([]string, 0, len(entries))
	for _, entry := range entries {
		corpus, err := os.ReadFile(filepath.Join(defaultDir, entry.Name()))
		if err != nil {
			fmt.Printf("Error reading corpus entry: %v\n", err)
		}
		res = append(res, string(corpus))
	}
	return res
}

func FuzzVMBasic(f *testing.F) {
	corpus := readCorpus()
	for _, elem := range corpus {
		f.Add([]byte(elem))
	}
	for i := range 255 {
		b := make([]byte, 32)
		for k := range 32 {
			b[k] = byte(i)
		}
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, a []byte) {
		Fuzz(a)
	})
}

func FuzzVMStateless(f *testing.F) {
	for i := range 255 {
		b := make([]byte, 32)
		for k := range 32 {
			b[k] = byte(i)
		}
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, a []byte) {
		FuzzStateless(a)
	})
}

func TestFuzzer(t *testing.T) {
	data := []byte("\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a\x5a")
	Fuzz([]byte(data))
}

func TestMinimizeProgram(t *testing.T) {
	// Only local test, should not be run in test pipeline
	data := "asdfadfasdfasdfasdfasdfasdfadsfldlafdsgoinsfandofaijdsf"
	f := filler.NewFiller([]byte(data))
	testMaker, _ := generator.GenerateProgram(f)
	if err := testMaker.Fill(nil); err != nil {
		panic(err)
	}
	// Save the test
	test := testMaker.ToGeneralStateTest("name")
	hashed := hash(testMaker.ToGeneralStateTest("hashName"))
	storeTest(test, makeDefaultPath(hashed, "name"))
	// minimize
	minimized, _, err := MinimizeProgram(testMaker)
	if err != nil {
		t.Error(err)
	}
	minTest := minimized.ToGeneralStateTest("name")
	_ = minTest
	fmt.Printf("%v", minTest)
	minHashed := hash(testMaker.ToGeneralStateTest("hashName"))
	storeTest(minTest, makeDefaultPath(minHashed, "name.min"))
}

func TestCreateStateTest(t *testing.T) {
	p := program.New()
	p.MstoreSmall([]byte{0x08}, 0x00) // base size 8
	p.MstoreSmall([]byte{0x51}, 0x20) // exponent size 6
	p.MstoreSmall([]byte{0x08}, 0x40) // modulo size 512
	base := common.FromHex("0xffffffffffffffff")
	p.Mstore(base, 0x60) // base
	exp := []byte{}
	for range 0x51 {
		exp = append(exp, 0xff)
	}
	p.Mstore(exp, 0x260) // exponent
	mod := common.FromHex("0xffffffffffffffff")
	p.Mstore(mod, 0x266) // modulo
	_, dest := p.Jumpdest()
	p.StaticCall(uint256.NewInt(0xffff), 0x5, 0, 0x466, 0, 0x466)
	p.Op(vm.POP)
	p.Jump(dest)

	code := p.Bytes()
	f := filler.NewFiller([]byte("\x5a\x5a\x5a\x5a\x5a\x5a\x5a"))
	testMaker := generator.CreateGstMaker(f, code)
	if err := testMaker.Fill(nil); err != nil {
		panic(err)
	}
	// Save the test
	test := testMaker.ToGeneralStateTest("statetest.json")
	storeTest(test, "statetest.json")
}
