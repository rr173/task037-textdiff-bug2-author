package diff

import (
	"reflect"
	"testing"
)

func TestBug2SplitLinesPreservesLeadingEmptyLine(t *testing.T) {
	want := []Line{
		{Content: "", Newline: true},
		{Content: "A", Newline: true},
	}
	if got := SplitLines("\nA\n"); !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitLines returned %#v, want %#v", got, want)
	}
}
