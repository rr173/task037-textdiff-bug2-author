package diff

// Line represents a single line of text without its trailing newline, along
// with whether that line was terminated by a newline in the source text.
// Two lines are equal only when both Content and Newline match: a final line
// "c" with no newline is distinct from "c\n", which is what makes the
// "No newline at end of file" marker computable from the edit script.
type Line struct {
	Content string
	Newline bool
}

// Equal reports whether two lines are identical for diff purposes.
func (l Line) Equal(o Line) bool {
	return l.Content == o.Content && l.Newline == o.Newline
}

// SplitLines splits s into Lines. An empty string yields no lines. A trailing
// newline does not produce an extra empty line; a final line without a
// trailing newline is marked Newline=false.
func SplitLines(s string) []Line {
	if s == "" {
		return nil
	}
	parts := stringsSplit(s, "\n")
	noNewline := true
	if len(parts) > 0 && lastChar(s) == '\n' {
		// strings.Split("a\n", "\n") == ["a", ""], so drop the trailing "".
		parts = parts[:len(parts)-1]
		noNewline = false
	}
	lines := make([]Line, len(parts))
	for i, p := range parts {
		lines[i] = Line{Content: p, Newline: true}
	}
	if noNewline && len(lines) > 0 {
		lines[len(lines)-1].Newline = false
	}
	return lines
}

func lastChar(s string) byte {
	if s == "" {
		return 0
	}
	return s[len(s)-1]
}

// stringsSplit is strings.Split without importing strings here, to keep the
// diff package free of an import that reads oddly next to pure algorithm
// code. It splits on a single-byte separator.
func stringsSplit(s, sep string) []string {
	if sep == "" {
		// Not used by callers; behave like strings.Split with empty sep.
		out := make([]string, 0, len(s)+1)
		for _, r := range s {
			out = append(out, string(r))
		}
		return out
	}
	var out []string
	start := 0
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			out = append(out, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	out = append(out, s[start:])
	return out
}

// OpKind classifies a single edit operation within the LCS edit script.
type OpKind int

const (
	Equal  OpKind = iota
	Delete        // a line present in old, removed in new
	Insert        // a line present in new, added relative to old
)

// Op is one step of the edit script. OldIdx indexes oldLines (for Equal,
// Delete); NewIdx indexes newLines (for Equal, Insert).
type Op struct {
	Kind   OpKind
	OldIdx int
	NewIdx int
}

// IsChange reports whether the op is a Delete or Insert.
func (o Op) IsChange() bool {
	return o.Kind != Equal
}

// Diff computes a minimal LCS-based edit script transforming oldLines into
// newLines. Ties in the dynamic program are broken by preferring a Delete
// (advance the old cursor) over an Insert, which makes output deterministic.
func Diff(oldLines, newLines []Line) []Op {
	n, m := len(oldLines), len(newLines)
	// dp[i][j] = length of LCS of oldLines[i:] and newLines[j:].
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if oldLines[i].Equal(newLines[j]) {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	ops := make([]Op, 0, n+m)
	i, j := 0, 0
	for i < n && j < m {
		if oldLines[i].Equal(newLines[j]) {
			ops = append(ops, Op{Kind: Equal, OldIdx: i, NewIdx: j})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			ops = append(ops, Op{Kind: Delete, OldIdx: i, NewIdx: j})
			i++
		} else {
			ops = append(ops, Op{Kind: Insert, OldIdx: i, NewIdx: j})
			j++
		}
	}
	for i < n {
		ops = append(ops, Op{Kind: Delete, OldIdx: i, NewIdx: j})
		i++
	}
	for j < m {
		ops = append(ops, Op{Kind: Insert, OldIdx: i, NewIdx: j})
		j++
	}
	return ops
}
