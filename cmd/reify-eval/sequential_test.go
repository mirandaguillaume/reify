package main

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWilsonHalfWidthShrinksWithN(t *testing.T) {
	wide := wilsonHalfWidth(5, 10)
	narrow := wilsonHalfWidth(50, 100)
	assert.Less(t, narrow, wide)
}

func TestShouldStop(t *testing.T) {
	assert.True(t, shouldStop(0, 8, 0.20, 100))
	assert.False(t, shouldStop(4, 8, 0.20, 100))
	assert.True(t, shouldStop(50, 100, 0.001, 100))
	assert.False(t, shouldStop(0, 1, 0.20, 100))
}

func TestWilsonHalfWidthBounds(t *testing.T) {
	w := wilsonHalfWidth(3, 10)
	assert.False(t, math.IsNaN(w))
	assert.Greater(t, w, 0.0)
}
