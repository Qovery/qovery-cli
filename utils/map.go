package utils

// Map applies a transformer function to each element in the slice
func Map[T, U any](slice []T, transformer func(T) U) []U {
	result := make([]U, len(slice))
	for i, item := range slice {
		result[i] = transformer(item)
	}
	return result
}
