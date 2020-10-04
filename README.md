# FuzzyVM [fuzz​ɛvm]

A framework to fuzz Ethereum Virtual Machine implementations.

## Environment
You need to have golang < 1.15 installed as go version 1.15 includes some 
changes that break go-fuzz.

## Install instructions

```shell
# Clone the repo to a place of your liking using
git clone git@github.com:MariusVanDerWijden/FuzzyVM.git
# Enter the repo
cd FuzzyVM
# Build the binary
go build
# Create the fuzz-test generator as follows:
./FuzzyVM --build
# Run the fuzzer
./FuzzyVM
```

