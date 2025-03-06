# FuzzyVM [fuzz​ɛvm]

A framework to fuzz Ethereum Virtual Machine implementations.
FuzzyVM creates state tests that can be used to differential fuzz EVM implementations against each other.
It only focus on the test generation part, the test execution is handled by [goevmlab](https://github.com/holiman/goevmlab).

## Environment
You need to have golang and go-ethereum installed

## Install instructions

```shell
# Clone the repo to a place of your liking using
git clone git@github.com:MariusVanDerWijden/FuzzyVM.git
# Enter the repo
cd FuzzyVM
# Build the binary
go build
# Create an initial corpus
./FuzzyVM corpus --count 100  
# Run the fuzzer
./FuzzyVM run
```

### Docker 
```shell
# Build the docker image
docker build . -t fuzzyvm -f Dockerfile
# Run the container and mount local directories
docker run -it -v ./out:/go/out -v ./corpus:/go/corpus fuzzyvm
# Start the fuzzer in the container
> ./FuzzyVM run

``` 

### Execute 
```shell
# we use goevmlab to execute the tests
docker run -it -v ./out:/fuzztmp --entrypoint bash holiman/omnifuzz
# now execute
runtest --gethbatch=/gethvm --nethbatch=/neth/nethtest --revme=/revme --besubatch=/evmtool/bin/evmtool /fuzztmp/\*/\*
```

# Corpus
It makes sense to create an initial corpus in order to improve the efficiency of the fuzzer.
You can generate corpus elements with `./FuzzyVM corpus --count N`, which will generate `N` corpus elements.

You might create corpus that is to big, you can minimize your corpus with `./FuzzyVM minCorpus`.

# Bench 
You can run a benchmark with `./FuzzyVM bench`. 
