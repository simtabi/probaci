package probaci

import "testing"

func TestDocLookup(t *testing.T) {
	for _, topic := range DocTopics() {
		if _, ok := Doc(topic); !ok {
			t.Errorf("topic %q should resolve to an embedded doc", topic)
		}
	}
	// Default + aliases.
	if md, ok := Doc(""); !ok || md == "" {
		t.Fatal("empty topic should default to overview")
	}
	if _, ok := Doc("config"); !ok {
		t.Fatal("alias 'config' should resolve")
	}
	if _, ok := Doc("nope"); ok {
		t.Fatal("unknown topic should not resolve")
	}
}
