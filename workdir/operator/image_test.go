package main

import "testing"

func TestEngineImage(t *testing.T) {
	cases := []struct {
		name, spec, related, want string
	}{
		{"spec wins", "registry.local/ruptura@sha256:aa", "mirror/ruptura@sha256:bb", "registry.local/ruptura@sha256:aa"},
		{"related image from the CSV", "", "mirror/ruptura@sha256:bb", "mirror/ruptura@sha256:bb"},
		{"built-in default", "", "", defaultImage},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(relatedImageEnv, c.related)
			if got := engineImage(c.spec); got != c.want {
				t.Errorf("engineImage(%q) = %q, want %q", c.spec, got, c.want)
			}
		})
	}
}
