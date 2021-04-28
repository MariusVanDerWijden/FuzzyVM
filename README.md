# FuzzyVM [fuzz​ɛvm]

A framework to fuzz Ethereum Virtual Machine implementations.

FuzzyVM uses two different processes. 
One process generates test cases and the other one executes them on different EVM implementations.
The traces of the EVM's are collected and compared against each other.
If a crasher is found, 

## Environment
You might need to have golang < 1.15 installed as go version 1.15 includes some 
changes that might break go-fuzz.

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

If the fuzzer breaks (or you shut it down) it might leave test cases laying around.
You can execute the test cases with `./FuzzyVM --exec`.

It makes sense to create an initial corpus in order to improve the efficiency of the fuzzer.
You can generate corpus elements with `./FuzzyVM --corpus N`.

## Config

FuzzyVM has to be configured to know which EVMs it should use.
You can specify the paths to the EVMs in a config file.
An example for the config file can be found in `config.toml`.
