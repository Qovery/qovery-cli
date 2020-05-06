package io

import "math/rand"

func RandomInt() int {
	max := 9999999
	min := 1000
	return rand.Intn(max-min) + min
}
