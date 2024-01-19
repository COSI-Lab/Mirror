package main

import (
	"bytes"
	"net/http"
	"sync"
	"time"

	"github.com/COSI-Lab/Mirror/logging"
)

type ProxyWriter struct {
	header http.Header
	buffer bytes.Buffer
	status int
}

func (p *ProxyWriter) Header() http.Header {
	return p.header
}

func (p *ProxyWriter) Write(bytes []byte) (int, error) {
	return p.buffer.Write(bytes)
}

func (p *ProxyWriter) WriteHeader(status int) {
	p.status = status
}

type CacheEntry struct {
	header http.Header
	body   []byte
	status int
	time   time.Time
}

func (c *CacheEntry) WriteTo(w http.ResponseWriter) (int, error) {
	header := w.Header()

	for k, v := range c.header {
		header[k] = v
	}

	if c.status != 0 {
		w.WriteHeader(c.status)
	}

	return w.Write(c.body)
}

// Caches the responses from the webserver
var cache = map[string]*CacheEntry{}
var cacheLock = &sync.RWMutex{}

func cachingMiddleware(next func(w http.ResponseWriter, r *http.Request)) http.Handler {
	if !webServerCache {
		logging.Info("Caching disabled")
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logging.Info(r.Method, r.URL.Path)
			next(w, r)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Check if the request is cached
		cacheLock.RLock()
		if entry, ok := cache[r.RequestURI]; ok && r.Method == "GET" {
			// Check that the cache entry is still valid
			if time.Since(entry.time) < time.Hour {
				// Send the cached response
				entry.WriteTo(w)
				cacheLock.RUnlock()
				logging.Info(r.Method, r.RequestURI, "in", time.Since(start), "; cached")
				return
			}
		}
		cacheLock.RUnlock()

		// Create a new response writer
		proxyWriter := &ProxyWriter{
			header: make(http.Header),
		}

		// Call the next handler
		next(proxyWriter, r)

		// Create the response cache entry
		entry := &CacheEntry{
			header: proxyWriter.header,
			body:   proxyWriter.buffer.Bytes(),
			status: proxyWriter.status,
			time:   time.Now(),
		}

		// Cache the response
		go func() {
			cacheLock.Lock()
			cache[r.RequestURI] = entry
			cacheLock.Unlock()
		}()

		// Send the response to client
		entry.WriteTo(w)
		logging.Info(r.Method, r.RequestURI, "in", time.Since(start))
	})
}
