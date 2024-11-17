package slices

func Contains[T comparable](slice []T, el T) bool {
	for _, e := range slice {
		if e == el {
			return true
		}
	}
	return false
}

func Equal[T comparable](slice []T, slice2 []T) bool {
	for i, el := range slice {
		if slice2[i] != el {
			return false
		}
	}
	return true
}

func Copy[T any](slice []T) []T {
	res := make([]T, 0, len(slice))
	copy(slice, res)
	return res
}
