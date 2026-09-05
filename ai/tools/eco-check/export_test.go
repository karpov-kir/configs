package ecocheck

// What the suite next door needs from inside this package. Bind a value here rather than retyping it
// in a case: a copy goes stale the next time the original is reworded, and the case stays green.
const (
	FindingCap       = findingCap
	SuppressedMarker = suppressedMarker
)
