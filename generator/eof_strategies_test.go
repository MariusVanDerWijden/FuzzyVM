package generator

import (
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
				panic(err)
			}
		}
	})
}

func TestEOFGenerator(t *testing.T) {
	data := []byte("\xa0\xfc")
	jt := vm.NewPragueEOFInstructionSetForTesting()
	fl := filler.NewFiller(data)
	container := RandomContainer(fl)
	//fmt.Printf("%x\n", container.MarshalBinary())
	newCon := new(vm.Container)
	if err := newCon.UnmarshalBinary(container.MarshalBinary(), true); err == nil {
		if err := newCon.ValidateCode(&jt, true); err == nil {
			panic(err)
		}
	}
}
