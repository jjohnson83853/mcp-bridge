package search

import "math"

// cosine returns the exact normalised cosine similarity of two finite vectors.
func cosine(a, b []float64) (float64, bool) {
	if len(a) == 0 || len(a) != len(b) {
		return 0, false
	}
	var dot, normA, normB float64
	for i := range a {
		if !finite(a[i]) || !finite(b[i]) {
			return 0, false
		}
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0, false
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB)), true
}

// validVector reports whether a vector is usable: exactly the model's
// configured dimensionality, all finite, and non-zero.
func validVector(vector []float64, dimensions int) bool {
	if len(vector) != dimensions {
		return false
	}
	nonZero := false
	for _, value := range vector {
		if !finite(value) {
			return false
		}
		if value != 0 {
			nonZero = true
		}
	}
	return nonZero
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
