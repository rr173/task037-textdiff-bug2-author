package diff

import "strings"

// Hunk is a contiguous region of the edit script rendered as unified diff.
// Lines holds the fully-rendered body lines, each already prefixed with
// ' ', '-', or '+', and includes any "\ No newline at end of file" marker
// lines interleaved in the correct position.
type Hunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []string
}

// Stats summarizes line-level change counts across the whole diff.
type Stats struct {
	Added   int
	Deleted int
	Hunks   int
}

// NoNewlineMarker is the exact marker line GNU diff emits for a final line
// that lacks a trailing newline.
const NoNewlineMarker = `\ No newline at end of file`

// ToHunks folds the edit script into a list of hunks using the GNU-style
// context rule: each change interval is expanded by `context` lines on both
// sides, then expanded intervals that touch or overlap are merged. Two
// changes separated by g unchanged lines end up in the same hunk iff
// g <= 2*context.
func ToHunks(ops []Op, oldLines, newLines []Line, context int) []Hunk {
	if context < 0 {
		context = 0
	}
	n := len(ops)

	// Collect change intervals in op-index space and expand by context.
	type interval struct{ lo, hi int }
	var ivs []interval
	i := 0
	for i < n {
		if !ops[i].IsChange() {
			i++
			continue
		}
		s := i
		for i < n && ops[i].IsChange() {
			i++
		}
		e := i
		lo := s - context
		if lo < 0 {
			lo = 0
		}
		hi := e + context
		if hi > n {
			hi = n
		}
		ivs = append(ivs, interval{lo, hi})
	}

	// Merge expanded intervals that touch or overlap: next.lo <= cur.hi.
	var merged []interval
	for _, iv := range ivs {
		if len(merged) > 0 && iv.lo <= merged[len(merged)-1].hi {
			if iv.hi > merged[len(merged)-1].hi {
				merged[len(merged)-1].hi = iv.hi
			}
		} else {
			merged = append(merged, iv)
		}
	}

	hunks := make([]Hunk, 0, len(merged))
	for _, iv := range merged {
		hunks = append(hunks, renderHunk(ops, iv.lo, iv.hi, oldLines, newLines))
	}
	return hunks
}

// renderHunk renders ops[lo:hi) as a single hunk, computing header counts and
// start line numbers per the GNU rules, and emitting "\ No newline at end of
// file" markers for the last line of either source when it lacks a newline.
func renderHunk(ops []Op, lo, hi int, oldLines, newLines []Line) Hunk {
	oldCount, newCount := 0, 0
	oldBefore, newBefore := 0, 0
	// Count old/new line occurrences before the hunk (for start numbering).
	for k := 0; k < lo; k++ {
		switch ops[k].Kind {
		case Equal:
			oldBefore++
			newBefore++
		case Delete:
			oldBefore++
		case Insert:
			newBefore++
		}
	}
	// Count within the hunk and build body lines.
	lastOld := len(oldLines) - 1
	lastNew := len(newLines) - 1
	var lines []string
	for k := lo; k < hi; k++ {
		op := ops[k]
		switch op.Kind {
		case Equal:
			oldCount++
			newCount++
			lines = append(lines, " "+oldLines[op.OldIdx].Content)
			if op.OldIdx == lastOld && !oldLines[op.OldIdx].Newline {
				// Equal line shared by both sides; old being last-without-newline
				// implies the same for new (lines are equal).
				lines = append(lines, NoNewlineMarker)
			}
		case Delete:
			oldCount++
			lines = append(lines, "-"+oldLines[op.OldIdx].Content)
			if op.OldIdx == lastOld && !oldLines[op.OldIdx].Newline {
				lines = append(lines, NoNewlineMarker)
			}
		case Insert:
			newCount++
			lines = append(lines, "+"+newLines[op.NewIdx].Content)
			if op.NewIdx == lastNew && !newLines[op.NewIdx].Newline {
				lines = append(lines, NoNewlineMarker)
			}
		}
	}
	return Hunk{
		OldStart: oldBefore + oneIfNonZero(oldCount),
		OldCount: oldCount,
		NewStart: newBefore + oneIfNonZero(newCount),
		NewCount: newCount,
		Lines:    lines,
	}
}

// oneIfNonZero returns 1 when n>0, else 0. This encodes the GNU rule: when a
// side has lines in the hunk, the start is the 1-based number of its first
// line; when it has none, the start is the count of that side's lines before
// the hunk (pointing at the preceding line, 0 at the file head).
func oneIfNonZero(n int) int {
	if n > 0 {
		return 1
	}
	return 0
}

// HeaderLine renders the hunk header "@@ -<os>,<oc> +<ns>,<nc> @@" applying
// the count omission rules: a count of 1 omits the ",1", a count of 0 keeps
// ",0", and any other count writes "start,count".
func (h Hunk) HeaderLine() string {
	return "@@ " + formatSide("-", h.OldStart, h.OldCount) + " " + formatSide("+", h.NewStart, h.NewCount) + " @@"
}

func formatSide(prefix string, start, count int) string {
	switch count {
	case 1:
		return prefix + itoa(start)
	default:
		return prefix + itoa(start) + "," + itoa(count)
	}
}

// itoa is a small dependency-free int-to-string (non-negative inputs only).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// Render produces the full unified diff text, the structured hunks, and line
// statistics for the given inputs. When there are no changes, unified is the
// empty string and hunks is empty.
func Render(old, new string, context int, oldLabel, newLabel string) (string, []Hunk, Stats) {
	oldLines := SplitLines(old)
	newLines := SplitLines(new)
	ops := Diff(oldLines, newLines)
	hunks := ToHunks(ops, oldLines, newLines, context)

	stats := Stats{Hunks: len(hunks)}
	for _, op := range ops {
		switch op.Kind {
		case Delete:
			stats.Deleted++
		case Insert:
			stats.Added++
		}
	}

	if len(hunks) == 0 {
		return "", hunks, stats
	}

	var b strings.Builder
	b.WriteString("--- " + oldLabel + "\n")
	b.WriteString("+++ " + newLabel + "\n")
	for _, h := range hunks {
		b.WriteString(h.HeaderLine())
		b.WriteString("\n")
		for _, l := range h.Lines {
			b.WriteString(l)
			b.WriteString("\n")
		}
	}
	return b.String(), hunks, stats
}
