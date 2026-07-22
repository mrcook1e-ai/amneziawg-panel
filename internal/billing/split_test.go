package billing

import (
	"testing"
)

func TestSplitEqual(t *testing.T) {
	got := splitEqual(1000, []string{"a", "b", "c"})
	if got["a"]+got["b"]+got["c"] != 1000 {
		t.Fatalf("sum %d != 1000", got["a"]+got["b"]+got["c"])
	}
	// 1000 / 3 = 333, remainder 1 → first sorted ID gets +1.
	if got["a"] != 334 || got["b"] != 333 || got["c"] != 333 {
		t.Fatalf("unexpected split: %+v", got)
	}
}

func sumAll(m map[string]int64) int64 {
	var s int64
	for _, v := range m {
		s += v
	}
	return s
}

func TestSplitByTrafficProportional(t *testing.T) {
	traffic := map[string]uint64{"a": 900, "b": 100}
	got := splitByTraffic(1000, traffic, []string{"a", "b"}, 0) // minSharePct 0 = чистая пропорция
	if sumAll(got) != 1000 {
		t.Fatalf("sum %d != 1000", sumAll(got))
	}
	if got["a"] != 900 || got["b"] != 100 {
		t.Fatalf("expected 900/100, got %+v", got)
	}
}

func TestSplitByTrafficFloorEnforced(t *testing.T) {
	// 2 плательщика, equal=500, floor=50%→250. b почти не пользовал → платит пол.
	traffic := map[string]uint64{"a": 9999, "b": 1}
	got := splitByTraffic(1000, traffic, []string{"a", "b"}, 50)
	if sumAll(got) != 1000 {
		t.Fatalf("sum %d != 1000", sumAll(got))
	}
	if got["b"] < 250 {
		t.Fatalf("b must pay at least floor 250, got %d", got["b"])
	}
	if got["a"] != 750 || got["b"] != 250 {
		t.Fatalf("expected a=750 b=250, got %+v", got)
	}
}

func TestSplitByTrafficZeroTrafficPayers(t *testing.T) {
	// a тяжёлый, b и c без трафика. floor=25% от equal(300)=75.
	traffic := map[string]uint64{"a": 1000, "b": 0, "c": 0}
	got := splitByTraffic(900, traffic, []string{"a", "b", "c"}, 25)
	if sumAll(got) != 900 {
		t.Fatalf("sum %d != 900", sumAll(got))
	}
	if got["b"] != 75 || got["c"] != 75 {
		t.Fatalf("zero-traffic payers must pay exactly floor 75, got b=%d c=%d", got["b"], got["c"])
	}
	if got["a"] != 750 {
		t.Fatalf("a must absorb the remainder 750, got %d", got["a"])
	}
}

func TestSplitByTrafficAllZeroFallsBackEqual(t *testing.T) {
	traffic := map[string]uint64{"a": 0, "b": 0}
	got := splitByTraffic(1000, traffic, []string{"a", "b"}, 25)
	if sumAll(got) != 1000 {
		t.Fatalf("sum %d != 1000", sumAll(got))
	}
	if got["a"] != 500 || got["b"] != 500 {
		t.Fatalf("all-zero must split equally 500/500, got %+v", got)
	}
}

func TestSplitByTrafficDeterministic(t *testing.T) {
	traffic := map[string]uint64{"a": 70, "b": 20, "c": 10}
	first := splitByTraffic(997, traffic, []string{"a", "b", "c"}, 25)
	second := splitByTraffic(997, traffic, []string{"a", "b", "c"}, 25)
	if sumAll(first) != 997 {
		t.Fatalf("sum %d != 997", sumAll(first))
	}
	for _, k := range []string{"a", "b", "c"} {
		if first[k] != second[k] {
			t.Fatalf("non-deterministic: %v vs %v", first, second)
		}
	}
}
