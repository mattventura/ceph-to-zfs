package util

func Map[In any, Out any](input []In, f func(In) Out) []Out {
	out := make([]Out, len(input))
	for i, in := range input {
		out[i] = f(in)
	}
	return out
}

func FindFirst[T any](input []T, f func(T) bool) (matching *T, found bool) {
	for _, in := range input {
		if f(in) {
			return &in, true
		}
	}
	return nil, false
}
