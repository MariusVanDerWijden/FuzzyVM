package generator

import (
	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/ethereum/go-ethereum/core/vm"
)

type eofGenerator struct{}

func (eofGenerator) Execute(env Environment) {
	container := RandomContainer(env.f)
	code := container.MarshalBinary()
	// Deploy code
	// call code
	_ = code
}

func (*eofGenerator) Importance() int {
	return 1
}

func RandomContainer(f *filler.Filler) *vm.Container {
	return randomSubContainer(f, 0, 0)
}

func randomSubContainer(f *filler.Filler, codeSize int, level int) *vm.Container {

	// Setup Function Metadata
	typeLen := f.SmallInt()
	types := make([]*vm.FunctionMetadata, 0, typeLen)
	for i := 0; i < typeLen; i++ {
		types = append(types, RandomFunctionMetadata(f))
	}
	// Setup Code
	codes := make([][]byte, 0, typeLen)
	for i := 0; i < typeLen; i++ {
		_, code := GenerateProgram(f)
		codes = append(codes, code)
		codeSize += len(code)
	}
	// Setup Data
	_, data := GenerateProgram(f)
	codeSize += len(data)
	// Setup Subcontainers
	subCLen := f.SmallInt()
	subContainers := make([]*vm.Container, 0, subCLen)
	subContainerCodes := make([][]byte, 0, subCLen)
	for i := 0; i < subCLen; i++ {
		if codeSize < maxCodeSize && level < maxContainerLevel {
			subC := randomSubContainer(f, codeSize, level+1)
			subContainers = append(subContainers, subC)
			subCode := subC.MarshalBinary()
			subContainerCodes = append(subContainerCodes, subCode)
			codeSize += len(subCode)
		}
	}

	return &vm.Container{
		Types:             types,
		Code:              codes,
		Data:              data,
		DataSize:          len(data),
		ContainerSections: subContainers,
		ContainerCode:     subContainerCodes,
	}
}

func RandomFunctionMetadata(f *filler.Filler) *vm.FunctionMetadata {
	// Create starting container with prob 1/2
	if f.Bool() {
		return &vm.FunctionMetadata{
			Input:          0,
			Output:         0x80,
			MaxStackHeight: uint16(f.Byte()),
		}
	}
	return &vm.FunctionMetadata{
		Input:          f.Byte(),
		Output:         f.Byte(),
		MaxStackHeight: f.Uint16(),
	}
}
