package gdc

import (
	"context"
	"fmt"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes gdc as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/gdc-cli/gdc"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then dereferences
// gdc:// URIs by routing to the operations Register installs. The same
// Domain also builds the standalone gdc binary (see cli.NewApp), so the
// binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the GDC driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "gdc",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "gdc",
			Short:  "A command line for the NCI Genomic Data Commons.",
			Long: `A command line for the NCI Genomic Data Commons (GDC).

gdc reads cancer genomics data from the NCI GDC API, which indexes
50k+ cases, 1.27M files, and 3.3M mutations across 91 cancer projects.
No API key required.`,
			Site: "https://portal.gdc.cancer.gov/",
			Repo: "https://github.com/tamnd/gdc-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{Name: "search", Group: "read", List: true,
		Summary: "Search cases by keyword (--site, --project, --limit, --offset)",
		Args:    []kit.Arg{{Name: "keyword", Help: "search keyword"}}}, searchCases)

	kit.Handle(app, kit.OpMeta{Name: "case", Group: "read", Single: true,
		Summary: "Get a single case by case_id", URIType: "case", Resolver: true,
		Args: []kit.Arg{{Name: "case-id", Help: "GDC case_id or submitter_id"}}}, getCase)

	kit.Handle(app, kit.OpMeta{Name: "files", Group: "read", List: true,
		Summary: "List files for a case (--limit)",
		Args:    []kit.Arg{{Name: "case-id", Help: "GDC case_id"}}}, listFiles)

	kit.Handle(app, kit.OpMeta{Name: "mutations", Group: "read", List: true,
		Summary: "List mutations in a gene (--limit)",
		Args:    []kit.Arg{{Name: "gene", Help: "gene symbol (e.g. TP53, BRCA1)"}}}, searchMutations)

	kit.Handle(app, kit.OpMeta{Name: "projects", Group: "read", List: true,
		Summary: "List all GDC cancer projects (--limit)"}, listProjects)
}

// newClient builds the GDC client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- inputs ---

type searchInput struct {
	Keyword string  `kit:"arg"          help:"search keyword"`
	Site    string  `kit:"flag"         help:"filter by primary site"`
	Project string  `kit:"flag"         help:"filter by project_id"`
	Limit   int     `kit:"flag,inherit" help:"max results"`
	Offset  int     `kit:"flag"         help:"pagination offset"`
	Client  *Client `kit:"inject"`
}

type caseInput struct {
	CaseID string  `kit:"arg"    help:"GDC case_id"`
	Client *Client `kit:"inject"`
}

type filesInput struct {
	CaseID string  `kit:"arg"          help:"GDC case_id"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Offset int     `kit:"flag"         help:"pagination offset"`
	Client *Client `kit:"inject"`
}

type mutationsInput struct {
	Gene   string  `kit:"arg"          help:"gene symbol"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Offset int     `kit:"flag"         help:"pagination offset"`
	Client *Client `kit:"inject"`
}

type projectsInput struct {
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Offset int     `kit:"flag"         help:"pagination offset"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func searchCases(ctx context.Context, in searchInput, emit func(*Case) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	cases, _, err := in.Client.SearchCases(ctx, in.Keyword, in.Site, in.Project, limit, in.Offset)
	if err != nil {
		return err
	}
	for i := range cases {
		if err := emit(&cases[i]); err != nil {
			return err
		}
	}
	return nil
}

func getCase(ctx context.Context, in caseInput, emit func(*Case) error) error {
	c, err := in.Client.GetCase(ctx, in.CaseID)
	if err != nil {
		return err
	}
	return emit(c)
}

func listFiles(ctx context.Context, in filesInput, emit func(*File) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	files, _, err := in.Client.ListFiles(ctx, in.CaseID, limit, in.Offset)
	if err != nil {
		return err
	}
	for i := range files {
		if err := emit(&files[i]); err != nil {
			return err
		}
	}
	return nil
}

func searchMutations(ctx context.Context, in mutationsInput, emit func(*Mutation) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	mutations, _, err := in.Client.SearchMutations(ctx, in.Gene, limit, in.Offset)
	if err != nil {
		return err
	}
	for i := range mutations {
		if err := emit(&mutations[i]); err != nil {
			return err
		}
	}
	return nil
}

func listProjects(ctx context.Context, in projectsInput, emit func(*Project) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	projects, _, err := in.Client.ListProjects(ctx, limit, in.Offset)
	if err != nil {
		return err
	}
	for i := range projects {
		if err := emit(&projects[i]); err != nil {
			return err
		}
	}
	return nil
}

// Classify turns any accepted input into the canonical (type, id).
// UUID-shaped inputs (contain "-" and len>30) are treated as case IDs;
// everything else is also classified as a case keyword search target.
func (Domain) Classify(input string) (string, string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", "", errs.Usage("gdc requires a case_id or keyword, got empty input")
	}
	return "case", s, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "case":
		return fmt.Sprintf("https://portal.gdc.cancer.gov/cases/%s", id), nil
	case "file":
		return fmt.Sprintf("https://portal.gdc.cancer.gov/files/%s", id), nil
	case "project":
		return fmt.Sprintf("https://portal.gdc.cancer.gov/projects/%s", id), nil
	default:
		return "", errs.Usage("gdc has no resource type %q", uriType)
	}
}
