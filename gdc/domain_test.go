package gdc

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring (mint, body, resolve), which need no network.
// The client's HTTP behaviour is covered in gdc_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "gdc" {
		t.Errorf("Scheme = %q, want gdc", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "gdc" {
		t.Errorf("Identity.Binary = %q, want gdc", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ in, typ, id string }{
		{"TCGA-AA-3994-01A-01D-A19A-10", "case", "TCGA-AA-3994-01A-01D-A19A-10"},
		{"3b4e1e83-5c78-4e69-9d17-9afaa5c60a5a", "case", "3b4e1e83-5c78-4e69-9d17-9afaa5c60a5a"},
		{"colon cancer", "case", "colon cancer"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil {
			t.Errorf("Classify(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if typ != tc.typ {
			t.Errorf("Classify(%q) type = %q, want %q", tc.in, typ, tc.typ)
		}
		if id != tc.id {
			t.Errorf("Classify(%q) id = %q, want %q", tc.in, id, tc.id)
		}
	}
}

func TestClassifyEmpty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestLocate(t *testing.T) {
	tests := []struct {
		typ  string
		id   string
		want string
	}{
		{"case", "abc-123", "https://portal.gdc.cancer.gov/cases/abc-123"},
		{"file", "file-456", "https://portal.gdc.cancer.gov/files/file-456"},
		{"project", "TCGA-COAD", "https://portal.gdc.cancer.gov/projects/TCGA-COAD"},
	}
	for _, tc := range tests {
		got, err := Domain{}.Locate(tc.typ, tc.id)
		if err != nil {
			t.Errorf("Locate(%q, %q) error: %v", tc.typ, tc.id, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Locate(%q, %q) = %q, want %q", tc.typ, tc.id, got, tc.want)
		}
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("mutation", "ssm-001")
	if err == nil {
		t.Fatal("expected error for unknown resource type")
	}
}

func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	c := &Case{
		ID:          "3b4e1e83-5c78-4e69-9d17-9afaa5c60a5a",
		SubmitterID: "TCGA-AA-0001",
		ProjectID:   "TCGA-COAD",
		PrimarySite: "Colon",
	}
	u, err := h.Mint(c)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	want := "gdc://case/3b4e1e83-5c78-4e69-9d17-9afaa5c60a5a"
	if u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}

	got, err := h.ResolveOn("gdc", "TCGA-AA-0001")
	if err != nil {
		t.Fatalf("ResolveOn: %v", err)
	}
	if got.String() != "gdc://case/TCGA-AA-0001" {
		t.Errorf("ResolveOn = %q, want gdc://case/TCGA-AA-0001", got.String())
	}
}
