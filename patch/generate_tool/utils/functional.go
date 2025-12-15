package utils

func Unique[T comparable](lists ...[]T) []T {
	var result []T
	cacheMap := make(map[T]bool)
	for _, arr := range lists {
		for _, it := range arr {
			if !cacheMap[it] {
				cacheMap[it] = true
				result = append(result, it)
			}
		}
	}
	return result
}

func Concat[T any](a []T, b ...T) []T {
	aLength := len(a)
	result := make([]T, aLength+len(b))
	for idx := range a {
		result[idx] = a[idx]
	}
	for idx := range b {
		result[aLength+idx] = b[idx]
	}
	return result
}

func Contains[T comparable](list []T, target T) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}
