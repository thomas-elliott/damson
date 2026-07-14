package metadata

import "testing"

func TestSortByPreferredStore(t *testing.T) {
	cands := []Candidate{
		{Title: "a", Store: StoreSketchfab},
		{Title: "b", Store: StoreItch},
		{Title: "c", Store: StoreUnity},
		{Title: "d", Store: StoreSketchfab},
	}
	SortByPreferredStore(cands, StoreUnity)
	if cands[0].Store != StoreUnity {
		t.Fatalf("preferred store not first: %+v", cands)
	}
	if cands[1].Store != StoreSketchfab || cands[2].Store != StoreItch || cands[3].Store != StoreSketchfab {
		t.Fatalf("non-preferred order not preserved: %+v", cands)
	}
}

func TestSortByPreferredStoreEmpty(t *testing.T) {
	cands := []Candidate{{Store: StoreItch}, {Store: StoreUnity}}
	SortByPreferredStore(cands, "")
	if cands[0].Store != StoreItch {
		t.Fatalf("empty preferred should not reorder: %+v", cands)
	}
}

func TestValidPreferredStore(t *testing.T) {
	if !ValidPreferredStore(StoreUnity) {
		t.Error("unity should be valid")
	}
	if ValidPreferredStore("nope") {
		t.Error("unknown store should be invalid")
	}
}
