// Package gdc is the library behind the gdc command line:
// the HTTP client, request shaping, and the typed data models for the
// NCI Genomic Data Commons (GDC), which indexes cancer genomics data:
// 50k+ cases, 1.27M files, 3.3M mutations across 91 cancer projects.
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public API throws under load.
package gdc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Host is the GDC API hostname this client talks to.
const Host = "api.gdc.cancer.gov"

// BaseURL is the root every request is built from.
const BaseURL = "https://api.gdc.cancer.gov"

const defaultUserAgent = "gdc-cli/0.1.0"

// Config holds the runtime settings for the GDC client.
type Config struct {
	BaseURL   string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
	UserAgent string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		Rate:      300 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
		UserAgent: defaultUserAgent,
	}
}

// Client talks to the GDC REST API.
type Client struct {
	cfg  Config
	http *http.Client
	last time.Time
}

// NewClient returns a Client using the given Config.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *Client) pace() {
	if c.cfg.Rate > 0 {
		if since := time.Since(c.last); since < c.cfg.Rate {
			time.Sleep(c.cfg.Rate - since)
		}
	}
	c.last = time.Now()
}

func (c *Client) get(ctx context.Context, rawURL string, out any) error {
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			d := time.Duration(attempt) * 500 * time.Millisecond
			if d > 5*time.Second {
				d = 5 * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(d):
			}
		}
		c.pace()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", c.cfg.UserAgent)
		req.Header.Set("Accept", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			if attempt < c.cfg.Retries {
				continue
			}
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			if attempt < c.cfg.Retries {
				continue
			}
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return fmt.Errorf("all retries exhausted")
}

// --- wire types (unexported) ---

type wirePagination struct {
	Total int `json:"total"`
	Count int `json:"count"`
	Page  int `json:"page"`
	From  int `json:"from"`
}

type wireDataResp struct {
	Data struct {
		Pagination wirePagination    `json:"pagination"`
		Hits       []json.RawMessage `json:"hits"`
	} `json:"data"`
}

type wireCase struct {
	ID          string `json:"id"`
	CaseID      string `json:"case_id"`
	SubmitterID string `json:"submitter_id"`
	PrimarySite string `json:"primary_site"`
	DiseaseType string `json:"disease_type"`
	Project     struct {
		ProjectID string `json:"project_id"`
		Name      string `json:"name"`
	} `json:"project"`
	Demographic struct {
		AgeAtIndex int    `json:"age_at_index"`
		Gender     string `json:"gender"`
		Race       string `json:"race"`
	} `json:"demographic"`
}

type wireFile struct {
	FileID       string `json:"file_id"`
	FileName     string `json:"file_name"`
	DataType     string `json:"data_type"`
	DataCategory string `json:"data_category"`
	DataFormat   string `json:"data_format"`
	FileSize     int64  `json:"file_size"`
	Access       string `json:"access"`
	Cases        []struct {
		CaseID  string `json:"case_id"`
		Project struct {
			ProjectID string `json:"project_id"`
		} `json:"project"`
	} `json:"cases"`
}

type wireMutation struct {
	SsmID           string `json:"ssm_id"`
	Chromosome      string `json:"chromosome"`
	StartPosition   int64  `json:"start_position"`
	EndPosition     int64  `json:"end_position"`
	ReferenceAllele string `json:"reference_allele"`
	TumorAllele     string `json:"tumor_allele"`
	MutationSubtype string `json:"mutation_subtype"`
	Consequence     []struct {
		Transcript struct {
			Gene struct {
				GeneID string `json:"gene_id"`
				Symbol string `json:"symbol"`
			} `json:"gene"`
		} `json:"transcript"`
	} `json:"consequence"`
}

type wireProject struct {
	ProjectID          string `json:"project_id"`
	Name               string `json:"name"`
	PrimarySite        string `json:"primary_site"`
	DBGapAccessionNum  string `json:"dbgap_accession_number"`
	Program            struct {
		Name string `json:"name"`
	} `json:"program"`
	Summary struct {
		CaseCount int `json:"case_count"`
		FileCount int `json:"file_count"`
	} `json:"summary"`
}

// --- public types ---

// Case is a single GDC cancer case record.
type Case struct {
	ID          string `json:"id"           kit:"id"`
	SubmitterID string `json:"submitter_id"`
	ProjectID   string `json:"project_id"`
	PrimarySite string `json:"primary_site"`
	DiseaseType string `json:"disease_type"`
	Age         int    `json:"age"`
	Gender      string `json:"gender"`
	Race        string `json:"race"`
}

// File is a single GDC file record.
type File struct {
	ID           string `json:"id"            kit:"id"`
	Name         string `json:"name"`
	DataType     string `json:"data_type"`
	DataCategory string `json:"data_category"`
	Format       string `json:"format"`
	SizeBytes    int64  `json:"size_bytes"`
	Access       string `json:"access"`
	CaseID       string `json:"case_id"`
	ProjectID    string `json:"project_id"`
}

// Mutation is a single GDC somatic mutation (SSM) record.
type Mutation struct {
	ID           string `json:"id"            kit:"id"`
	Chromosome   string `json:"chromosome"`
	StartPos     int64  `json:"start_pos"`
	EndPos       int64  `json:"end_pos"`
	RefAllele    string `json:"ref_allele"`
	TumorAllele  string `json:"tumor_allele"`
	MutationType string `json:"mutation_type"`
	GeneID       string `json:"gene_id"`
	GeneSymbol   string `json:"gene_symbol"`
}

// Project is a single GDC cancer project record.
type Project struct {
	ID          string `json:"id"           kit:"id"`
	Name        string `json:"name"`
	Program     string `json:"program"`
	PrimarySite string `json:"primary_site"`
	Cases       int    `json:"cases"`
	Files       int    `json:"files"`
}

// toCase maps a wireCase to a public Case.
func toCase(w wireCase) *Case {
	id := w.CaseID
	if id == "" {
		id = w.ID
	}
	return &Case{
		ID:          id,
		SubmitterID: w.SubmitterID,
		ProjectID:   w.Project.ProjectID,
		PrimarySite: w.PrimarySite,
		DiseaseType: w.DiseaseType,
		Age:         w.Demographic.AgeAtIndex,
		Gender:      w.Demographic.Gender,
		Race:        w.Demographic.Race,
	}
}

// toFile maps a wireFile to a public File.
func toFile(w wireFile) *File {
	var caseID, projectID string
	if len(w.Cases) > 0 {
		caseID = w.Cases[0].CaseID
		projectID = w.Cases[0].Project.ProjectID
	}
	return &File{
		ID:           w.FileID,
		Name:         w.FileName,
		DataType:     w.DataType,
		DataCategory: w.DataCategory,
		Format:       w.DataFormat,
		SizeBytes:    w.FileSize,
		Access:       w.Access,
		CaseID:       caseID,
		ProjectID:    projectID,
	}
}

// toMutation maps a wireMutation to a public Mutation.
func toMutation(w wireMutation) *Mutation {
	var geneID, geneSymbol string
	if len(w.Consequence) > 0 {
		geneID = w.Consequence[0].Transcript.Gene.GeneID
		geneSymbol = w.Consequence[0].Transcript.Gene.Symbol
	}
	return &Mutation{
		ID:           w.SsmID,
		Chromosome:   w.Chromosome,
		StartPos:     w.StartPosition,
		EndPos:       w.EndPosition,
		RefAllele:    w.ReferenceAllele,
		TumorAllele:  w.TumorAllele,
		MutationType: w.MutationSubtype,
		GeneID:       geneID,
		GeneSymbol:   geneSymbol,
	}
}

// toProject maps a wireProject to a public Project.
func toProject(w wireProject) *Project {
	return &Project{
		ID:          w.ProjectID,
		Name:        w.Name,
		Program:     w.Program.Name,
		PrimarySite: w.PrimarySite,
		Cases:       w.Summary.CaseCount,
		Files:       w.Summary.FileCount,
	}
}

// filterJSON builds a GDC filter JSON string for a single equality check.
func filterJSON(field, value string) string {
	b, _ := json.Marshal(map[string]any{
		"op": "and",
		"content": []map[string]any{
			{
				"op": "=",
				"content": map[string]any{
					"field": field,
					"value": value,
				},
			},
		},
	})
	return string(b)
}

// filterAnd builds a GDC filter JSON string for multiple equality checks.
func filterAnd(pairs [][2]string) string {
	content := make([]map[string]any, 0, len(pairs))
	for _, p := range pairs {
		content = append(content, map[string]any{
			"op": "=",
			"content": map[string]any{
				"field": p[0],
				"value": p[1],
			},
		})
	}
	b, _ := json.Marshal(map[string]any{
		"op":      "and",
		"content": content,
	})
	return string(b)
}

const caseFields = "case_id,submitter_id,project.project_id,project.name,primary_site,disease_type,demographic.age_at_index,demographic.gender,demographic.race"
const fileFields = "file_id,file_name,data_type,data_category,data_format,file_size,access,state,cases.case_id,cases.project.project_id"
const mutationFields = "ssm_id,chromosome,start_position,end_position,reference_allele,tumor_allele,mutation_subtype,consequence.transcript.gene.gene_id,consequence.transcript.gene.symbol"
const projectFields = "project_id,name,program.name,primary_site,dbgap_accession_number,releasable,released,state,summary.case_count,summary.file_count"

// SearchCases searches cases by keyword and optional site/project filters.
func (c *Client) SearchCases(ctx context.Context, keyword, site, project string, limit, from int) ([]Case, int, error) {
	if limit <= 0 {
		limit = 20
	}
	u := fmt.Sprintf("%s/cases?size=%d&from=%d&format=json&fields=%s",
		c.cfg.BaseURL, limit, from, url.QueryEscape(caseFields))
	if keyword != "" {
		u += "&q=" + url.QueryEscape(keyword)
	}

	var filters [][2]string
	if site != "" {
		filters = append(filters, [2]string{"primary_site", site})
	}
	if project != "" {
		filters = append(filters, [2]string{"project.project_id", project})
	}
	if len(filters) > 0 {
		u += "&filters=" + url.QueryEscape(filterAnd(filters))
	}

	var resp wireDataResp
	if err := c.get(ctx, u, &resp); err != nil {
		return nil, 0, err
	}
	cases := make([]Case, 0, len(resp.Data.Hits))
	for _, raw := range resp.Data.Hits {
		var w wireCase
		if err := json.Unmarshal(raw, &w); err != nil {
			continue
		}
		cases = append(cases, *toCase(w))
	}
	return cases, resp.Data.Pagination.Total, nil
}

// GetCase fetches a single case by its case_id.
func (c *Client) GetCase(ctx context.Context, caseID string) (*Case, error) {
	u := fmt.Sprintf("%s/cases/%s?format=json&fields=%s",
		c.cfg.BaseURL, url.PathEscape(caseID), url.QueryEscape(caseFields))
	// GDC single-entity endpoint returns {"data": {...}}
	var resp struct {
		Data wireCase `json:"data"`
	}
	// First try as a direct single-entity response.
	body := bytes.Buffer{}
	_ = body // we decode directly
	if err := c.get(ctx, u, &resp); err != nil {
		return nil, err
	}
	return toCase(resp.Data), nil
}

// ListFiles lists files for a given case_id.
func (c *Client) ListFiles(ctx context.Context, caseID string, limit, from int) ([]File, int, error) {
	if limit <= 0 {
		limit = 20
	}
	u := fmt.Sprintf("%s/files?size=%d&from=%d&format=json&fields=%s&filters=%s",
		c.cfg.BaseURL, limit, from, url.QueryEscape(fileFields),
		url.QueryEscape(filterJSON("cases.case_id", caseID)))
	var resp wireDataResp
	if err := c.get(ctx, u, &resp); err != nil {
		return nil, 0, err
	}
	files := make([]File, 0, len(resp.Data.Hits))
	for _, raw := range resp.Data.Hits {
		var w wireFile
		if err := json.Unmarshal(raw, &w); err != nil {
			continue
		}
		files = append(files, *toFile(w))
	}
	return files, resp.Data.Pagination.Total, nil
}

// SearchMutations lists mutations for a gene symbol.
func (c *Client) SearchMutations(ctx context.Context, geneSymbol string, limit, from int) ([]Mutation, int, error) {
	if limit <= 0 {
		limit = 20
	}
	u := fmt.Sprintf("%s/ssms?size=%d&from=%d&format=json&fields=%s&filters=%s",
		c.cfg.BaseURL, limit, from, url.QueryEscape(mutationFields),
		url.QueryEscape(filterJSON("consequence.transcript.gene.symbol", geneSymbol)))
	var resp wireDataResp
	if err := c.get(ctx, u, &resp); err != nil {
		return nil, 0, err
	}
	mutations := make([]Mutation, 0, len(resp.Data.Hits))
	for _, raw := range resp.Data.Hits {
		var w wireMutation
		if err := json.Unmarshal(raw, &w); err != nil {
			continue
		}
		mutations = append(mutations, *toMutation(w))
	}
	return mutations, resp.Data.Pagination.Total, nil
}

// ListProjects lists all GDC cancer projects.
func (c *Client) ListProjects(ctx context.Context, limit, from int) ([]Project, int, error) {
	if limit <= 0 {
		limit = 20
	}
	u := fmt.Sprintf("%s/projects?size=%d&from=%d&format=json&fields=%s",
		c.cfg.BaseURL, limit, from, url.QueryEscape(projectFields))
	var resp wireDataResp
	if err := c.get(ctx, u, &resp); err != nil {
		return nil, 0, err
	}
	projects := make([]Project, 0, len(resp.Data.Hits))
	for _, raw := range resp.Data.Hits {
		var w wireProject
		if err := json.Unmarshal(raw, &w); err != nil {
			continue
		}
		projects = append(projects, *toProject(w))
	}
	return projects, resp.Data.Pagination.Total, nil
}
