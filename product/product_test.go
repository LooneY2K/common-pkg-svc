package product

import "testing"

func TestParseReturnsEveryProduct(t *testing.T) {
	for _, p := range All() {
		got, err := Parse(string(p))
		if err != nil || got != p {
			t.Errorf("Parse(%q) = %q, %v", p, got, err)
		}
	}
}

func TestParseRejectsAnythingElse(t *testing.T) {
	for _, name := range []string{"", "Orbit", "PUCKOOPS", "drive", " orbit"} {
		if _, err := Parse(name); err == nil {
			t.Errorf("Parse(%q) accepted a name that is not a product", name)
		}
	}
}

func TestScopesAreNamedAfterTheProduct(t *testing.T) {
	if Puckoops.Database() != "puckoops" || Puckoops.FGAStore() != "puckoops" || Puckoops.ObjectPrefix() != "puckoops/" {
		t.Errorf("puckoops scopes = %q %q %q", Puckoops.Database(), Puckoops.FGAStore(), Puckoops.ObjectPrefix())
	}
	if Orbit.Database() != "orbit" || Orbit.ObjectPrefix() != "orbit/" {
		t.Errorf("orbit scopes = %q %q", Orbit.Database(), Orbit.ObjectPrefix())
	}
}

func TestObjectKeyStaysUnderItsProduct(t *testing.T) {
	cases := map[string][]string{
		"puckoops/nodes/a/b/report.pdf": {"nodes", "a", "b", "report.pdf"},
		"puckoops/orbit/workspaces/x":   {"..", "..", "orbit", "workspaces", "x"},
		"puckoops/nodes/x":              {"/nodes/", "../nodes", "x"},
	}
	for want, parts := range cases {
		if got := Puckoops.ObjectKey(parts...); got != want {
			t.Errorf("ObjectKey(%q) = %q, want %q", parts, got, want)
		}
	}
}
