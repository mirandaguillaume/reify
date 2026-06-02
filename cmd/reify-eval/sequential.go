package main

import "math"

// z for a 95% two-sided confidence interval.
const wilsonZ = 1.96

// wilsonHalfWidth returns the half-width of the Wilson score interval for
// k successes in n trials. n must be > 0.
func wilsonHalfWidth(k, n int) float64 {
	if n <= 0 {
		return 1.0
	}
	p := float64(k) / float64(n)
	z := wilsonZ
	nn := float64(n)
	denom := 1 + z*z/nn
	centre := p + z*z/(2*nn)
	margin := z * math.Sqrt(p*(1-p)/nn+z*z/(4*nn*nn))
	lo := (centre - margin) / denom
	hi := (centre + margin) / denom
	return (hi - lo) / 2
}

// shouldStop decides whether a cell's sampling can stop: either the
// Wilson interval is tight enough, or nMax is reached. Requires a minimum
// of 4 samples so an early lucky streak does not stop prematurely.
func shouldStop(k, n int, targetHalfWidth float64, nMax int) bool {
	if n >= nMax {
		return true
	}
	if n < 4 {
		return false
	}
	return wilsonHalfWidth(k, n) <= targetHalfWidth
}
