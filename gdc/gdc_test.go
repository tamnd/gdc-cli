package gdc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// testServer spins up an httptest.Server with the given mux and returns a
// zero-rate, zero-retry Client pointed at it.
func testServer(t *testing.T, mux *http.ServeMux) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 0
	return srv, NewClient(cfg)
}

// gdcResp wraps hits into the standard GDC envelope.
func gdcResp(hits []any, total int) any {
	rawHits := make([]json.RawMessage, len(hits))
	for i, h := range hits {
		b, _ := json.Marshal(h)
		rawHits[i] = json.RawMessage(b)
	}
	return map[string]any{
		"data": map[string]any{
			"pagination": map[string]any{
				"total": total,
				"count": len(hits),
				"page":  1,
				"from":  0,
			},
			"hits": rawHits,
		},
	}
}

func TestSearchCases(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cases", func(w http.ResponseWriter, r *http.Request) {
		hits := []any{
			wireCase{
				CaseID:      "case-001",
				SubmitterID: "TCGA-AA-0001",
				Project:     struct{ ProjectID string "json:\"project_id\""; Name string "json:\"name\"" }{ProjectID: "TCGA-COAD", Name: "Colon Adenocarcinoma"},
				PrimarySite: "Colon",
				DiseaseType: "Adenomas and Adenocarcinomas",
				Demographic: struct{ AgeAtIndex int "json:\"age_at_index\""; Gender string "json:\"gender\""; Race string "json:\"race\"" }{AgeAtIndex: 55, Gender: "male", Race: "white"},
			},
			wireCase{
				CaseID:      "case-002",
				SubmitterID: "TCGA-AA-0002",
				Project:     struct{ ProjectID string "json:\"project_id\""; Name string "json:\"name\"" }{ProjectID: "TCGA-COAD", Name: "Colon Adenocarcinoma"},
				PrimarySite: "Colon",
				DiseaseType: "Adenomas and Adenocarcinomas",
			},
		}
		json.NewEncoder(w).Encode(gdcResp(hits, 50))
	})
	_, client := testServer(t, mux)
	cases, total, err := client.SearchCases(context.Background(), "colon", "", "", 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 50 {
		t.Errorf("total = %d, want 50", total)
	}
	if len(cases) != 2 {
		t.Fatalf("len = %d, want 2", len(cases))
	}
	if cases[0].ID != "case-001" {
		t.Errorf("ID = %q, want case-001", cases[0].ID)
	}
	if cases[0].ProjectID != "TCGA-COAD" {
		t.Errorf("ProjectID = %q, want TCGA-COAD", cases[0].ProjectID)
	}
	if cases[0].Age != 55 {
		t.Errorf("Age = %d, want 55", cases[0].Age)
	}
	if cases[1].ID != "case-002" {
		t.Errorf("ID = %q, want case-002", cases[1].ID)
	}
}

func TestGetCase(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/case-001", func(w http.ResponseWriter, r *http.Request) {
		wc := wireCase{
			CaseID:      "case-001",
			SubmitterID: "TCGA-AA-0001",
			PrimarySite: "Bladder",
			DiseaseType: "Transitional Cell Carcinoma",
		}
		json.NewEncoder(w).Encode(map[string]any{"data": wc})
	})
	_, client := testServer(t, mux)
	c, err := client.GetCase(context.Background(), "case-001")
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "case-001" {
		t.Errorf("ID = %q, want case-001", c.ID)
	}
	if c.PrimarySite != "Bladder" {
		t.Errorf("PrimarySite = %q, want Bladder", c.PrimarySite)
	}
}

func TestListFiles(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		hits := []any{
			wireFile{
				FileID:       "file-001",
				FileName:     "TCGA-AA-0001.bam",
				DataType:     "Aligned Reads",
				DataCategory: "Sequencing Reads",
				DataFormat:   "BAM",
				FileSize:     1024 * 1024 * 500,
				Access:       "controlled",
				Cases: []struct {
					CaseID  string `json:"case_id"`
					Project struct {
						ProjectID string `json:"project_id"`
					} `json:"project"`
				}{
					{CaseID: "case-001", Project: struct {
						ProjectID string `json:"project_id"`
					}{ProjectID: "TCGA-COAD"}},
				},
			},
			wireFile{
				FileID:   "file-002",
				FileName: "TCGA-AA-0001.vcf.gz",
				FileSize: 1024 * 100,
				Access:   "open",
			},
		}
		json.NewEncoder(w).Encode(gdcResp(hits, 15))
	})
	_, client := testServer(t, mux)
	files, total, err := client.ListFiles(context.Background(), "case-001", 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 15 {
		t.Errorf("total = %d, want 15", total)
	}
	if len(files) != 2 {
		t.Fatalf("len = %d, want 2", len(files))
	}
	if files[0].ID != "file-001" {
		t.Errorf("ID = %q, want file-001", files[0].ID)
	}
	if files[0].SizeBytes != 1024*1024*500 {
		t.Errorf("SizeBytes = %d, want %d", files[0].SizeBytes, 1024*1024*500)
	}
	if files[0].CaseID != "case-001" {
		t.Errorf("CaseID = %q, want case-001", files[0].CaseID)
	}
	if files[1].ID != "file-002" {
		t.Errorf("ID = %q, want file-002", files[1].ID)
	}
}

func TestSearchMutations(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ssms", func(w http.ResponseWriter, r *http.Request) {
		hits := []any{
			wireMutation{
				SsmID:           "ssm-001",
				Chromosome:      "17",
				StartPosition:   7674220,
				EndPosition:     7674220,
				ReferenceAllele: "C",
				TumorAllele:     "T",
				MutationSubtype: "Single base substitution",
				Consequence: []struct {
					Transcript struct {
						Gene struct {
							GeneID string `json:"gene_id"`
							Symbol string `json:"symbol"`
						} `json:"gene"`
					} `json:"transcript"`
				}{
					{Transcript: struct {
						Gene struct {
							GeneID string `json:"gene_id"`
							Symbol string `json:"symbol"`
						} `json:"gene"`
					}{Gene: struct {
						GeneID string `json:"gene_id"`
						Symbol string `json:"symbol"`
					}{GeneID: "ENSG00000141510", Symbol: "TP53"}}},
				},
			},
			wireMutation{
				SsmID:      "ssm-002",
				Chromosome: "17",
			},
		}
		json.NewEncoder(w).Encode(gdcResp(hits, 8234))
	})
	_, client := testServer(t, mux)
	mutations, total, err := client.SearchMutations(context.Background(), "TP53", 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 8234 {
		t.Errorf("total = %d, want 8234", total)
	}
	if len(mutations) != 2 {
		t.Fatalf("len = %d, want 2", len(mutations))
	}
	if mutations[0].ID != "ssm-001" {
		t.Errorf("ID = %q, want ssm-001", mutations[0].ID)
	}
	if mutations[0].Chromosome != "17" {
		t.Errorf("Chromosome = %q, want 17", mutations[0].Chromosome)
	}
	if mutations[0].GeneSymbol != "TP53" {
		t.Errorf("GeneSymbol = %q, want TP53", mutations[0].GeneSymbol)
	}
}

func TestListProjects(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) {
		hits := []any{
			wireProject{
				ProjectID:   "TCGA-COAD",
				Name:        "Colon Adenocarcinoma",
				PrimarySite: "Colon",
				Program:     struct{ Name string "json:\"name\"" }{Name: "TCGA"},
				Summary:     struct{ CaseCount int "json:\"case_count\""; FileCount int "json:\"file_count\"" }{CaseCount: 524, FileCount: 6918},
			},
			wireProject{
				ProjectID: "TCGA-BLCA",
				Name:      "Bladder Urothelial Carcinoma",
				Program:   struct{ Name string "json:\"name\"" }{Name: "TCGA"},
				Summary:   struct{ CaseCount int "json:\"case_count\""; FileCount int "json:\"file_count\"" }{CaseCount: 412, FileCount: 5234},
			},
		}
		json.NewEncoder(w).Encode(gdcResp(hits, 91))
	})
	_, client := testServer(t, mux)
	projects, total, err := client.ListProjects(context.Background(), 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 91 {
		t.Errorf("total = %d, want 91", total)
	}
	if len(projects) != 2 {
		t.Fatalf("len = %d, want 2", len(projects))
	}
	if projects[0].ID != "TCGA-COAD" {
		t.Errorf("ID = %q, want TCGA-COAD", projects[0].ID)
	}
	if projects[0].Name != "Colon Adenocarcinoma" {
		t.Errorf("Name = %q, want Colon Adenocarcinoma", projects[0].Name)
	}
	if projects[0].Cases != 524 {
		t.Errorf("Cases = %d, want 524", projects[0].Cases)
	}
}

func TestRetryOn503(t *testing.T) {
	var hits int
	mux := http.NewServeMux()
	mux.HandleFunc("/cases", func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		json.NewEncoder(w).Encode(gdcResp([]any{}, 0))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5

	client := NewClient(cfg)
	start := time.Now()
	_, _, err := client.SearchCases(context.Background(), "", "", "", 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}
