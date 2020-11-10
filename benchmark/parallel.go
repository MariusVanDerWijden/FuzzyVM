package benchmark

import (
	"time"

	"github.com/MariusVanDerWijden/FuzzyVM/executor"
	"github.com/holiman/goevmlab/evms"
)

func parallel(N int) (time.Duration, error) {
	evms.Docker = false
	executor.ParallelEVMS = true
	return execLinearMultiple(N, false)
}

func parallelBatch(N int) (time.Duration, error) {
	evms.Docker = false
	executor.ParallelEVMS = true
	return execLinearMultiple(N, true)
}

func parallelDocker(N int) (time.Duration, error) {
	evms.Docker = true
	executor.ParallelEVMS = true
	return execLinearMultiple(N, false)
}

func parallelBatchDocker(N int) (time.Duration, error) {
	evms.Docker = true
	executor.ParallelEVMS = true
	return execLinearMultiple(N, true)
}
