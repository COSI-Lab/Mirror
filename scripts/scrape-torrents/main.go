package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gocolly/colly"
)

func main() {
	depthPtr := flag.Int("depth", 1, "max traversal depth")
	flag.Parse()

	if len(flag.Args()) != 1 {
		fmt.Fprintln(os.Stderr, "missing url")
		os.Exit(1)
	}

	url := flag.Args()[0]

	// Instantiate default collector
	c := colly.NewCollector(
		// MaxDepth is 1, so only the links on the scraped page
		// is visited, and no further links are followed
		colly.MaxDepth(*depthPtr),
	)

	// On every a element which has href attribute call callback
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		// Print link
		fmt.Println(link)
		// Visit link found on page
		e.Request.Visit(link)
	})

	// Before making a request print "Visiting ..."
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL.String())
	})

	c.Visit(url)
}
