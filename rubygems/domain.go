// Package rubygems exposes rubygems.org as a kit Domain.
//
// A multi-domain host (ant) enables it with a single blank import:
//
//	import _ "github.com/tamnd/rubygems-cli/rubygems"
//
// The same Domain also builds the standalone rubygems binary.
package rubygems

import (
	"context"
	"time"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the rubygems driver.
type Domain struct{}

// Info describes the scheme, hostnames, and binary identity.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "rubygems",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "rubygems",
			Short:  "Ruby gem registry search and info (rubygems.org)",
			Long: `rubygems fetches gem search results, gem info, and version history
from the public RubyGems.org API. No API key required.`,
			Site: Host,
			Repo: "https://github.com/tamnd/rubygems-cli",
		},
	}
}

// Register installs the client factory and operations onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// search: find gems matching a query
	kit.Handle(app, kit.OpMeta{
		Name:    "search",
		Group:   "read",
		List:    true,
		Summary: "Search for Ruby gems",
		Args:    []kit.Arg{{Name: "query", Help: "search query"}},
	}, searchOp)

	// gem: fetch info for a single gem
	kit.Handle(app, kit.OpMeta{
		Name:    "gem",
		Group:   "read",
		Single:  true,
		Summary: "Show info for a Ruby gem",
		Args:    []kit.Arg{{Name: "name", Help: "gem name"}},
	}, gemOp)

	// versions: list versions of a gem
	kit.Handle(app, kit.OpMeta{
		Name:    "versions",
		Group:   "read",
		List:    true,
		Summary: "List versions of a Ruby gem",
		Args:    []kit.Arg{{Name: "name", Help: "gem name"}},
	}, versionsOp)

	// deps: list reverse dependencies
	kit.Handle(app, kit.OpMeta{
		Name:    "deps",
		Group:   "read",
		List:    true,
		Summary: "List gems that depend on this gem (reverse dependencies)",
		Args:    []kit.Arg{{Name: "name", Help: "gem name"}},
	}, depsOp)
}

// newClient bridges kit.Config to Config.
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
	Query  string        `kit:"arg" help:"search query"`
	Limit  int           `kit:"flag,inherit" help:"max results"`
	Delay  time.Duration `kit:"flag,inherit" help:"minimum spacing between requests"`
	Client *Client       `kit:"inject"`
}

type gemInput struct {
	Name   string  `kit:"arg" help:"gem name"`
	Client *Client `kit:"inject"`
}

type versionsInput struct {
	Name   string        `kit:"arg" help:"gem name"`
	Limit  int           `kit:"flag,inherit" help:"max results"`
	Delay  time.Duration `kit:"flag,inherit" help:"minimum spacing between requests"`
	Client *Client       `kit:"inject"`
}

type depsInput struct {
	Name   string        `kit:"arg" help:"gem name"`
	Limit  int           `kit:"flag,inherit" help:"max results"`
	Delay  time.Duration `kit:"flag,inherit" help:"minimum spacing between requests"`
	Client *Client       `kit:"inject"`
}

// --- handlers ---

func searchOp(ctx context.Context, in searchInput, emit func(Gem) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	items, err := in.Client.Search(ctx, in.Query, limit)
	if err != nil {
		return mapErr(err)
	}
	for _, item := range items {
		if err := emit(item); err != nil {
			return err
		}
	}
	return nil
}

func gemOp(ctx context.Context, in gemInput, emit func(Gem) error) error {
	g, err := in.Client.GemInfo(ctx, in.Name)
	if err != nil {
		return mapErr(err)
	}
	return emit(g)
}

func versionsOp(ctx context.Context, in versionsInput, emit func(Version) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	items, err := in.Client.Versions(ctx, in.Name, limit)
	if err != nil {
		return mapErr(err)
	}
	for _, item := range items {
		if err := emit(item); err != nil {
			return err
		}
	}
	return nil
}

func depsOp(ctx context.Context, in depsInput, emit func(ReverseDep) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	items, err := in.Client.ReverseDeps(ctx, in.Name, limit)
	if err != nil {
		return mapErr(err)
	}
	for _, item := range items {
		if err := emit(item); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver ---

// Classify maps a gem name to (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("empty rubygems reference")
	}
	return "gem", input, nil
}

// Locate returns the rubygems.org page URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "gem":
		return "https://rubygems.org/gems/" + id, nil
	default:
		return "", errs.Usage("rubygems has no resource type %q", uriType)
	}
}

// mapErr converts a library error into a kit error kind.
func mapErr(err error) error {
	return err
}
