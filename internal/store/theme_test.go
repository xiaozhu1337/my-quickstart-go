package store

import "testing"

func TestStoreThemeSaveLoadAndReopen(t *testing.T) {
	for _, theme := range []string{"dark", "light", "midnight", "forest", "rose"} {
		t.Run(theme, func(t *testing.T) {
			s := openTestStore(t)
			want := sampleState()
			want.Settings.Theme = theme
			want.Normalize()
			if want.Settings.Theme != theme {
				t.Fatalf("normalized theme: want %q, got %q", theme, want.Settings.Theme)
			}
			if err := s.Save(want); err != nil {
				t.Fatal(err)
			}
			got, err := s.Load()
			if err != nil {
				t.Fatal(err)
			}
			assertState(t, want, got)
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(s.Path())
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			got, err = reopened.Load()
			if err != nil {
				t.Fatal(err)
			}
			assertState(t, want, got)
			got.Normalize()
			assertState(t, want, got)
		})
	}
}
