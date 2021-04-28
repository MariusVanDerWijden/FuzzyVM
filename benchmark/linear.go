package benchmark

import (
	"fmt"
	"time"

	"github.com/MariusVanDerWijden/FuzzyVM/executor"
	"github.com/MariusVanDerWijden/FuzzyVM/generator"
	"github.com/holiman/goevmlab/evms"
)

// testGeneration generates N programs.
func testGeneration(N int) (time.Duration, error) {
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

// verify verifies a programs result N times.
func verify(N int) (time.Duration, error) {
	outDir, _, err := createTempDirs()
	if err != nil {
		return time.Nanosecond, err
	}
	name := "BenchTest"
	if err := generateTest(name, outDir); err != nil {
		return time.Nanosecond, err
	}
	name = fmt.Sprintf("%v/%v.json", outDir, name)
	exc := newExecutor()
	out, err := exc.ExecuteTest(name)
	if err != nil {
		return time.Nanosecond, err
	}
	start := time.Now()
	for i := 0; i < N; i++ {
		if !exc.Verify(name, out) {
			return time.Nanosecond, fmt.Errorf("Verification failed: %v", name)
		}
	}
	return time.Since(start), nil
}

// execution executes a program N times.
func execution(N int) (time.Duration, error) {
	outDir, crashers, err := createTempDirs()
	if err != nil {
		return time.Nanosecond, err
	}
	name := "BenchTest"
	if err := generateTest(name, outDir); err != nil {
		return time.Nanosecond, err
	}
	name = fmt.Sprintf("%v.json", name)
	exc := newExecutor()
	start := time.Now()
	for i := 0; i < N; i++ {
		exc.ExecuteFullTest(outDir, crashers, name, false)
	}
	return time.Since(start), nil
}

func linear(N int, threadlimit int) (time.Duration, error) {
	return execMultiple(N, threadlimit)
}

func linearDocker(N int, threadlimit int) (time.Duration, error) {
	// evms.Docker = true
	return execMultiple(N, threadlimit)
}

// execMultiple creates N tests and executes them in multiple threads.
func execMultiple(N int, threadlimit int) (time.Duration, error) {
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
	exc := newExecutor()
	start := time.Now()
	if err := exc.Execute(outDir, crashers, threadlimit); err != nil {
		return time.Nanosecond, err
	}
	return time.Since(start), nil
}

func newExecutor() *executor.Executor {
	// TODO add meaningful vms for benchmarks
	return executor.NewExecutor([]evms.Evm{}, false)
}
