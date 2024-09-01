package generator

import (
	"fmt"
	"testing"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/core/vm"
)

func FuzzEOFGenerator(f *testing.F) {
	jt := vm.NewPragueEOFInstructionSetForTesting()
	f.Fuzz(func(t *testing.T, data []byte) {
		fl := filler.NewFiller(data)
		container := RandomContainer(fl)
		newCon := new(vm.Container)
		if err := newCon.UnmarshalBinary(container.MarshalBinary(), true); err == nil {
			if err := newCon.ValidateCode(&jt, true); err == nil {
				if len(newCon.Code) > 0 && len(newCon.Code[0]) > 10 {
					panic(newCon.Code)
				}
			}
		}
	})
}

func TestEOFGenerator(t *testing.T) {
	data := []byte("01\xbe\x00\x01 \xfe")
	jt := vm.NewPragueEOFInstructionSetForTesting()
	fl := filler.NewFiller(data)
	container := RandomContainer(fl)
	//fmt.Printf("%x\n", container.MarshalBinary())
	newCon := new(vm.Container)
	if err := newCon.UnmarshalBinary(container.MarshalBinary(), true); err == nil {
		if err := newCon.ValidateCode(&jt, true); err == nil {
			fmt.Println(container.Code)
		}
	}
}
