package benchmark

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/MariusVanDerWijden/FuzzyVM/generator"
	"github.com/holiman/goevmlab/fuzzing"
)

// RunFullBench runs a full benchmark with N runs.
func RunFullBench(N int) {
	time, err := testGeneration(N)
	// Basic building blocks
	printResult("BenchmarkTestGeneration", time, err)
	time, err = execution(N)
	printResult("BenchmarkExecution", time, err)
	time, err = verify(N)
	printResult("BenchmarkVerification", time, err)
	// single thread execution
	time, err = single(N)
	printResult("BenchmarkSingle", time, err)
	time, err = singleBatch(N)
	printResult("BenchmarkSingleBatch", time, err)
	time, err = singleDocker(N)
	printResult("BenchmarkSingleDocker", time, err)
	time, err = singleBatchDocker(N)
	printResult("BenchmarkSingleBatchDocker", time, err)

	// parallel execution linear evms (structure 3.1)
	time, err = linear(N)
	printResult("BenchmarkLinear", time, err)
	time, err = linearBatch(N)
	printResult("BenchmarkLinearBatch", time, err)
	time, err = linearDocker(N)
	printResult("BenchmarkLinearDocker", time, err)
	time, err = linearBatchDocker(N)
	printResult("BenchmarkLinearBatchDocker", time, err)

	// parallel execution parallel evms (structure 3.2)
	time, err = parallel(N)
	printResult("BenchmarkParallel", time, err)
	time, err = parallelBatch(N)
	printResult("BenchmarkParallelBatch", time, err)
	time, err = parallelDocker(N)
	printResult("BenchmarkParallelDocker", time, err)

	time, err = parallelBatchDocker(N)
	printResult("BenchmarkParallelBatchDocker", time, err)

	// pipe strategy besu
	time, err = piping(N)
	printResult("BenchmarkPipeStrategy", time, err)
}

func printResult(name string, time time.Duration, err error) {
	if err != nil {
		fmt.Printf("Benchmark %v produced error: %v\n", name, err)
		return
	}
	fmt.Printf("Benchmark %v took %v \n", name, time.String())
}

// generates and stores a test
func generateTest(name, outputDir string) error {
	f, err := newFiller()
	if err != nil {
		return err
	}
	// Generate a test
	prog, _ := generator.GenerateProgram(f)
	if err := prog.Fill(nil); err != nil {
		return err
	}
	// Save the test
	test := prog.ToGeneralStateTest(name)
	if err := storeTest(test, name, outputDir); err != nil {
		return err
	}
	return nil
}

// storeTest saves a testcase to disk
func storeTest(test *fuzzing.GeneralStateTest, testName, outputDir string) error {
	path := fmt.Sprintf("%v/%v.json", outputDir, testName)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0755)
	if err != nil {
		return fmt.Errorf("Could not open test file: %v", err)
	}
	defer f.Close()
	// Write to file
	encoder := json.NewEncoder(f)
	if err = encoder.Encode(test); err != nil {
		return fmt.Errorf("Could not encode state test: %v", err)
	}
	return nil
}

func createTempDirs() (string, string, error) {
	outputDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", "", err
	}
	crashers, err := ioutil.TempDir("", "")
	if err != nil {
		return "", "", err
	}
	return outputDir, crashers, nil
}

func newFiller() (*filler.Filler, error) {
	rand.Seed(12345)
	rnd := make([]byte, 40)
	if _, err := rand.Read(rnd); err != nil {
		return nil, err
	}
	return filler.NewFiller(rnd), nil
}
