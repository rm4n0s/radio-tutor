package timeline

// Apply maps the progress u (0 at the previous key, 1 at this key) of a
// segment to the eased progress. u is clamped to [0, 1]. For EaseStep the
// result is 0 for every u below 1, so the value jumps when the key is
// reached. An empty or unknown Ease is linear.
func (e Ease) Apply(u float64) float64 {
	if u <= 0 {
		return 0
	}
	if u >= 1 {
		return 1
	}
	switch e {
	case EaseIn:
		return u * u * u
	case EaseOut:
		v := 1 - u
		return 1 - v*v*v
	case EaseInOut:
		if u < 0.5 {
			return 4 * u * u * u
		}
		v := -2*u + 2
		return 1 - v*v*v/2
	case EaseStep:
		return 0
	}
	return u
}
