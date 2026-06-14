package rubygems_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tamnd/rubygems-cli/rubygems"
)

const fakeSearchJSON = `[{"name":"sinatra","downloads":225461234,"version":"4.2.1","authors":"Blake Mizerany","info":"Sinatra is a DSL for quickly creating web applications.","homepage_uri":"https://sinatrarb.com","source_code_uri":"https://github.com/sinatra/sinatra"},{"name":"sinatra-contrib","downloads":15234567,"version":"4.2.1","authors":"Blake Mizerany","info":"Collection of useful Sinatra extensions.","homepage_uri":"","source_code_uri":""}]`

const fakeGemJSON = `{"name":"rails","downloads":592314234,"version":"8.1.3","authors":"David Heinemeier Hansson","info":"Ruby on Rails is a full-stack web framework.","homepage_uri":"https://rubyonrails.org","source_code_uri":"https://github.com/rails/rails"}`

func newTestClient(ts *httptest.Server) *rubygems.Client {
	cfg := rubygems.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return rubygems.NewClient(cfg)
}

func TestSearchSendsUA(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = fmt.Fprint(w, fakeSearchJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Search(context.Background(), "sinatra", 0)
	if err != nil {
		t.Fatal(err)
	}
	if gotUA == "" {
		t.Error("User-Agent not sent")
	}
}

func TestSearchParsesItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeSearchJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.Search(context.Background(), "sinatra", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].Name != "sinatra" {
		t.Errorf("items[0].Name = %q, want sinatra", items[0].Name)
	}
	if items[0].Downloads != 225461234 {
		t.Errorf("items[0].Downloads = %d, want 225461234", items[0].Downloads)
	}
	if items[0].SourceURI != "https://github.com/sinatra/sinatra" {
		t.Errorf("items[0].SourceURI = %q, want https://github.com/sinatra/sinatra", items[0].SourceURI)
	}
	if items[1].Name != "sinatra-contrib" {
		t.Errorf("items[1].Name = %q, want sinatra-contrib", items[1].Name)
	}
}

func TestSearchLimitRespected(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeSearchJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.Search(context.Background(), "sinatra", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Errorf("len(items) = %d, want 1", len(items))
	}
}

func TestSearchRetriesOn503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = fmt.Fprint(w, fakeSearchJSON)
	}))
	defer ts.Close()

	cfg := rubygems.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 3
	c := rubygems.NewClient(cfg)

	_, err := c.Search(context.Background(), "sinatra", 0)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}

func TestGemInfoParsesItem(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeGemJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	gem, err := c.GemInfo(context.Background(), "rails")
	if err != nil {
		t.Fatal(err)
	}
	if gem.Name != "rails" {
		t.Errorf("gem.Name = %q, want rails", gem.Name)
	}
	if gem.Downloads != 592314234 {
		t.Errorf("gem.Downloads = %d, want 592314234", gem.Downloads)
	}
	if gem.Version != "8.1.3" {
		t.Errorf("gem.Version = %q, want 8.1.3", gem.Version)
	}
}

const fakeReverseDepsJSON = `["activeadmin","devise","pundit","cancancan","ransack"]`

func TestReverseDepsParses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeReverseDepsJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.ReverseDeps(context.Background(), "rails", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 5 {
		t.Fatalf("len(items) = %d, want 5", len(items))
	}
	if items[0].Name != "activeadmin" {
		t.Errorf("items[0].Name = %q, want activeadmin", items[0].Name)
	}
	if items[0].Rank != 1 {
		t.Errorf("items[0].Rank = %d, want 1", items[0].Rank)
	}
	if items[0].URL != "https://rubygems.org/gems/activeadmin" {
		t.Errorf("items[0].URL = %q", items[0].URL)
	}
}

func TestReverseDepsLimitRespected(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeReverseDepsJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.ReverseDeps(context.Background(), "rails", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Errorf("len(items) = %d, want 3", len(items))
	}
}
