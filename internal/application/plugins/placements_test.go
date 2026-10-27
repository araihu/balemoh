package plugins

import "testing"

func TestSemanticPlacementOptions(t *testing.T) {
	a := PlacementBody{Options: []byte(`{"n":1,"a":[true,null,{"n":1000000000000000001}]}`)}
	b := a
	b.Options = []byte(`{"a":[true,null,{"n":1000000000000000001.0}],"n":1.0}`)
	if !SamePlacement(a, b) {
		t.Fatal("semantically identical JSON differs")
	}
	b.Options = []byte(`{"n":1,"a":[true,null,{"n":1000000000000000002}]}`)
	if SamePlacement(a, b) {
		t.Fatal("lost integer precision")
	}
}
