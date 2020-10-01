# FuzzyVM [fuzz​ɛvm]

A framework to fuzz Ethereum Virtual Machine implementations.

## Install instructions

```shell
# Clone the repo to a place of your liking using
git clone git@github.com:MariusVanDerWijden/FuzzyVM.git
# Enter the repo
cd FuzzyVM
# Create the fuzz-test generator as follows:
cd fuzzer
CGOEnabled=false go-fuzz-build
# Build the binary
go build
# Run the fuzzer
./FuzzyVM
```

