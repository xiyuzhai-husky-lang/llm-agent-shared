package highlight

// span is a byte range over the original UTF-8 source. Captures never split a rune.
type span struct {
	start, end int
	priority   uint
	class      string
}

// activeSpans selects the most specific capture, then the later query pattern.
type activeSpans []span

func (s activeSpans) Len() int { return len(s) }
func (s activeSpans) Less(i, j int) bool {
	a, b := s[i], s[j]
	if a.end-a.start != b.end-b.start {
		return a.end-a.start < b.end-b.start
	}
	return a.priority > b.priority
}
func (s activeSpans) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s *activeSpans) Push(v any)   { *s = append(*s, v.(span)) }
func (s *activeSpans) Pop() any     { a := *s; v := a[len(a)-1]; *s = a[:len(a)-1]; return v }
