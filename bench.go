package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/MariusVanDerWijden/FuzzyVM/executor"
)

func BenchmarkVerify(N int) (time.Duration, error) {
	name := generateTest()
	out, err := executor.ExecuteTest(name)
	if err != nil {
		return time.Nanosecond, err
	}
	start := time.Now()
	for i := 0; i < N; i++ {
		if executor.Verify(name, out) {
			return time.Nanosecond, errors.New(fmt.Sprintf("Verification failed: %v", name))
		}
	}
	return time.Since(start), nil
}
