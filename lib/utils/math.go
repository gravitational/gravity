package utils

// Min returns the smaller of two numbers
func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// Max returns the greater of two numbers
func Max(x, y int) int {
	if x < y {
		return y
	}
	return x
}

// MaxInt64 returns the greater of (x, y).
func MaxInt64(x, y uint64) uint64 {
	if x < y {
		return y
	}
	return x
}
