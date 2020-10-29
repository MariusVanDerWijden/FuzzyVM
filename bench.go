package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	"github.com/MariusVanDerWijden/FuzzyVM/executor"
	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/MariusVanDerWijden/FuzzyVM/generator"
	"github.com/holiman/goevmlab/evms"
	"github.com/holiman/goevmlab/fuzzing"
)

// RunFullBench runs a full benchmark with N runs.
func RunFullBench(N int) {
	time, err := BenchmarkTestGeneration(N)
	printResult("BenchmarkTestGeneration", time, err)
	time, err = BenchmarkExecution(N)
	printResult("BenchmarkExecution", time, err)
	time, err = BenchmarkVerify(N)
	printResult("BenchmarkVerification", time, err)
	time, err = BenchmarkMultiple(N)
	printResult("BenchmarkMultiple", time, err)
	time, err = BenchmarkMultipleBatch(N)
	printResult("BenchmarkMultipleBatch", time, err)
	time, err = BenchmarkMultipleDocker(N)
	printResult("BenchmarkMultipleDocker", time, err)
	time, err = BenchmarkMultipleBatchDocker(N)
	printResult("BenchmarkMultipleBatchDocker", time, err)
}

func printResult(name string, time time.Duration, err error) {
	if err != nil {
		fmt.Printf("Benchmark %v produced error: %v\n", name, err)
		return
	}
	fmt.Printf("Benchmark %v took %v \n", name, time.String())
}

// BenchmarkTestGeneration generates N programs.
func BenchmarkTestGeneration(N int) (time.Duration, error) {
	f, err := newFiller()
	if err != nil {
		return time.Nanosecond, err
	}
	start := time.Now()
	for i := 0; i < N; i++ {
		generator.GenerateProgram(f)
	}
	return time.Since(start), nil
}

// BenchmarkVerify verifies a programs result N times.
func BenchmarkVerify(N int) (time.Duration, error) {
	outDir, _, err := createTempDirs()
	if err != nil {
		return time.Nanosecond, err
	}
	name := "BenchTest"
	if err := generateTest(name, outDir); err != nil {
		return time.Nanosecond, err
	}
	name = fmt.Sprintf("%v/%v.json", outDir, name)
	out, err := executor.ExecuteTest(name)
	if err != nil {
		return time.Nanosecond, err
	}
	start := time.Now()
	for i := 0; i < N; i++ {
		if !executor.Verify(name, out) {
			return time.Nanosecond, fmt.Errorf("Verification failed: %v", name)
		}
	}
	return time.Since(start), nil
}

// BenchmarkExecution executes a program N times.
func BenchmarkExecution(N int) (time.Duration, error) {
	outDir, crashers, err := createTempDirs()
	if err != nil {
		return time.Nanosecond, err
	}
	name := "BenchTest"
	if err := generateTest(name, outDir); err != nil {
		return time.Nanosecond, err
	}
	name = fmt.Sprintf("%v.json", name)
	executor.PrintTrace = false
	start := time.Now()
	for i := 0; i < N; i++ {
		executor.ExecuteFullTest(outDir, crashers, name, false)
	}
	return time.Since(start), nil
}

// BenchmarkMultipleBatch runs a batch of N tests.
func BenchmarkMultipleBatch(N int) (time.Duration, error) {
	return benchMultiple(N, true)
}

// BenchmarkMultiple runs N tests in sequence.
func BenchmarkMultiple(N int) (time.Duration, error) {
	return benchMultiple(N, false)
}

// BenchmarkMultipleDocker runs N tests in sequence on a docker container.
func BenchmarkMultipleDocker(N int) (time.Duration, error) {
	evms.Docker = true
	return benchMultiple(N, false)
}

// BenchmarkMultipleBatch runs a batch of N tests.
func BenchmarkMultipleBatchDocker(N int) (time.Duration, error) {
	evms.Docker = true
	return benchMultiple(N, true)
}

func benchMultiple(N int, batch bool) (time.Duration, error) {
	outDir, crashers, err := createTempDirs()
	if err != nil {
		return time.Nanosecond, err
	}
	var names []string
	for i := 0; i < N; i++ {
		name := fmt.Sprintf("BenchTest-%v", i)
		if err := generateTest(name, outDir); err != nil {
			return time.Nanosecond, err
		}
		names = append(names, fmt.Sprintf("%v.json", name))
	}
	executor.PrintTrace = false
	start := time.Now()
	if batch {
		if err := executor.ExecuteFullBatch(outDir, crashers, names, false); err != nil {
			return time.Nanosecond, err
		}
	} else {
		for _, n := range names {
			if err := executor.ExecuteFullTest(outDir, crashers, n, false); err != nil {
				return time.Nanosecond, err
			}
		}
	}
	return time.Since(start), nil
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
