package baseinfo

import "testing"

func TestParseCIDRValid(t *testing.T) {
	ip, prefix, err := parseCIDR("1.2.3.0/24")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip != "1.2.3.0" || prefix != 24 {
		t.Fatalf("unexpected parse result: ip=%s prefix=%d", ip, prefix)
	}
}

func TestParseCIDRInvalidCases(t *testing.T) {
	cases := []string{
		"",
		"1.2.3.4",
		"1.2.3.4/",
		"/24",
		"bad/24",
		"1.2.3.4/x",
		"1.2.3.4/33",
		"1.2.3.4/-1",
	}
	for _, c := range cases {
		_, _, err := parseCIDR(c)
		if err == nil {
			t.Fatalf("expected error for %q", c)
		}
	}
}

func TestParseCIDRFromHEMalformedJSON(t *testing.T) {
	if got := parseCIDRFromHE("not-json"); got != "" {
		t.Fatalf("expected empty cidr, got %q", got)
	}
}

func TestParseCIDRFromBGPToolsNoMatch(t *testing.T) {
	if got := parseCIDRFromBGPTools("<html>no prefix here</html>"); got != "" {
		t.Fatalf("expected empty cidr, got %q", got)
	}
}
