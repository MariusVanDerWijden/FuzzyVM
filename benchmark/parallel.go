package benchmark

import (
	"fmt"
	"time"

	"github.com/MariusVanDerWijden/FuzzyVM/executor"
	"github.com/holiman/goevmlab/evms"
)

func parallel(N int) (time.Duration, error) {
	evms.Docker = false
	executor.ParallelEVMS = true
	return execMultiple(N, false)
}

func parallelBatch(N int) (time.Duration, error) {
	evms.Docker = false
	executor.ParallelEVMS = true
	return execMultiple(N, true)
}

func parallelDocker(N int) (time.Duration, error) {
	evms.Docker = true
	executor.ParallelEVMS = true
	return execMultiple(N, false)
}

func parallelBatchDocker(N int) (time.Duration, error) {
	evms.Docker = true
	executor.ParallelEVMS = true
	return execMultiple(N, true)
}

func linear(N int) (time.Duration, error) {
	evms.Docker = false
	executor.ParallelEVMS = false
	return execMultiple(N, false)
}

func linearBatch(N int) (time.Duration, error) {
	evms.Docker = false
	executor.ParallelEVMS = false
	return execMultiple(N, true)
}

func linearDocker(N int) (time.Duration, error) {
	evms.Docker = true
	executor.ParallelEVMS = false
	return execMultiple(N, false)
}

func linearBatchDocker(N int) (time.Duration, error) {
	evms.Docker = true
	executor.ParallelEVMS = false
	return execMultiple(N, true)
}

// execMultiple creates N tests and executes them in multiple threads.
func execMultiple(N int, batch bool) (time.Duration, error) {
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
		if err := executor.ExecuteBatch(outDir, crashers); err != nil {
			return time.Nanosecond, err
		}
	} else {
		if err := executor.Execute(outDir, crashers); err != nil {
			return time.Nanosecond, err
		}
	}
	return time.Since(start), nil
}
