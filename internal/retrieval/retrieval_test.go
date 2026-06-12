package retrieval

import (
	"testing"

	"github.com/vllnt/dig/internal/vector"
)

func TestParseMode(t *testing.T) {
	for in, want := range map[string]Mode{
		"":       ModeFTS,
		"fts":    ModeFTS,
		"vector": ModeVector,
		"hybrid": ModeHybrid,
	} {
		got, err := ParseMode(in)
		if err != nil || got != want {
			t.Fatalf("ParseMode(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := ParseMode("semantic"); err == nil {
		t.Fatal("unknown mode must error")
	}
}

func r(path string) vector.Result { return vector.Result{Path: path, Blob: "b:" + path} }

func TestFuseRRF(t *testing.T) {
	fts := []vector.Result{r("a"), r("b"), r("c")}
	vec := []vector.Result{r("b"), r("d"), r("a")}

	out := Fuse(fts, vec, 10)
	if len(out) != 4 {
		t.Fatalf("want 4 fused results, got %d: %+v", len(out), out)
	}
	// b: 1/(60+2) + 1/(60+1) — top by appearing high in both rankings.
	// a: 1/(60+1) + 1/(60+3) — second.
	if out[0].Path != "b" || out[1].Path != "a" {
		t.Fatalf("RRF order wrong: %+v", out)
	}
	// Singles rank below doubles.
	if out[2].Path != "c" && out[2].Path != "d" {
		t.Fatalf("singles should follow doubles: %+v", out)
	}
	// Scores monotonically non-increasing.
	for i := 1; i < len(out); i++ {
		if out[i].Score > out[i-1].Score {
			t.Fatalf("scores not sorted at %d: %+v", i, out)
		}
	}
}

func TestFuseRespectsLimit(t *testing.T) {
	fts := []vector.Result{r("a"), r("b"), r("c"), r("d")}
	out := Fuse(fts, nil, 2)
	if len(out) != 2 || out[0].Path != "a" {
		t.Fatalf("limit not respected: %+v", out)
	}
}

func TestFuseDeterministicTieBreak(t *testing.T) {
	// Identical single-list ranks → tie on score → path order decides, stably.
	a := Fuse([]vector.Result{r("z")}, []vector.Result{r("y")}, 10)
	b := Fuse([]vector.Result{r("z")}, []vector.Result{r("y")}, 10)
	if a[0].Path != b[0].Path || a[0].Path != "y" {
		t.Fatalf("tie break not deterministic: %+v vs %+v", a, b)
	}
}
