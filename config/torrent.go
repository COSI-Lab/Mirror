package config

// ScrapeTarget is the struct that represents a single upstream to scrape .torrent files from
type ScrapeTarget struct {
	URL   string `json:"url"`
	Delay int    `json:"delay"`
	Depth int    `json:"depth"`
}
