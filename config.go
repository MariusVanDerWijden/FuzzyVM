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

package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"reflect"
	"unicode"

	"github.com/holiman/goevmlab/evms"
	"github.com/naoina/toml"
)

func getVMsFromConfig(file string) ([]evms.Evm, error) {
	conf, err := loadConfig(file)
	if err != nil {
		return []evms.Evm{}, err
	}
	var vms []evms.Evm
	for _, s := range conf.Geth {
		vms = append(vms, evms.NewGethEVM(s))
	}
	for _, s := range conf.Nethermind {
		vms = append(vms, evms.NewNethermindVM(s))
	}
	for _, s := range conf.Besu {
		vms = append(vms, evms.NewBesuVM(s))
	}
	for _, s := range conf.OpenEthereum {
		vms = append(vms, evms.NewParityVM(s))
	}
	for _, s := range conf.Aleth {
		vms = append(vms, evms.NewAlethVM(s))
	}
	return vms, nil
}

type config struct {
	Geth         []string
	Besu         []string
	OpenEthereum []string
	Nethermind   []string
	Aleth        []string
}

func loadConfig(file string) (*config, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	conf := &config{}
	err = tomlSettings.NewDecoder(bufio.NewReader(f)).Decode(conf)
	// Add file name to errors that have a line number.
	if _, ok := err.(*toml.LineError); ok {
		err = errors.New(file + ", " + err.Error())
	}
	return conf, err
}

// These settings ensure that TOML keys use the same names as Go struct fields.
var tomlSettings = toml.Config{
	NormFieldName: func(rt reflect.Type, key string) string {
		return key
	},
	FieldToKey: func(rt reflect.Type, field string) string {
		return field
	},
	MissingField: func(rt reflect.Type, field string) error {
		link := ""
		if unicode.IsUpper(rune(rt.Name()[0])) && rt.PkgPath() != "main" {
			link = fmt.Sprintf(", see https://godoc.org/%s#%s for available fields", rt.PkgPath(), rt.Name())
		}
		return fmt.Errorf("field '%s' is not defined in %s%s", field, rt.String(), link)
	},
}
