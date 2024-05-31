package main

import "runtime"

// AdjustMaxProcs adjusts the maximum number of CPUs that can be executing.
func AdjustMaxProcs() int {
	n := runtime.NumCPU()
	runtime.GOMAXPROCS(n)

	return n
}
