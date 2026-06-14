package rubygems

// Gem is one Ruby gem record from rubygems.org.
type Gem struct {
	Rank        int    `json:"rank"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Downloads   int64  `json:"downloads"`
	Authors     string `json:"authors"`
	Info        string `json:"info"`
	HomepageURI string `json:"homepage_uri"`
	SourceURI   string `json:"source_uri"`
}

// Version is one version record for a Ruby gem.
type Version struct {
	Rank      int    `json:"rank"`
	Number    string `json:"number"`
	CreatedAt string `json:"created_at"`
	Downloads int64  `json:"downloads"`
	RubyVer   string `json:"ruby_version"`
}

// rawGem is the wire format from the RubyGems search and gem-info endpoints.
type rawGem struct {
	Name          string `json:"name"`
	Downloads     int64  `json:"downloads"`
	Version       string `json:"version"`
	Authors       string `json:"authors"`
	Info          string `json:"info"`
	HomepageURI   string `json:"homepage_uri"`
	SourceCodeURI string `json:"source_code_uri"`
}

// rawVersion is the wire format from the versions endpoint.
type rawVersion struct {
	Number         string `json:"number"`
	CreatedAt      string `json:"created_at"`
	DownloadsCount int64  `json:"downloads_count"`
	RubyVersion    string `json:"ruby_version"`
}

// ReverseDep is a gem that reverse-depends on the queried gem.
type ReverseDep struct {
	Rank int    `json:"rank"`
	Name string `json:"name"`
	URL  string `json:"url"`
}
