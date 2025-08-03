package cache

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http/httpguts"
)

type CacheEntry struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	StoredAt   time.Time
	InitialAge time.Time // Time when the entry was first cached
}

type Cache struct {
	rw      sync.RWMutex
	entries map[string]*CacheEntry
}

func NewCache() *Cache {
	return &Cache{
		entries: make(map[string]*CacheEntry),
	}
}

func (c *Cache) Get(key string) (*CacheEntry, bool) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	entry, found := c.entries[key]
	return entry, found
}
func (c *Cache) Set(key string, entry *CacheEntry) {
	c.rw.Lock()
	defer c.rw.Unlock()
	c.entries[key] = entry
}
func (c *Cache) Delete(key string) {
	c.rw.Lock()
	defer c.rw.Unlock()
	delete(c.entries, key)
}

func parseDirective(directive string) (string, string) {
	token := ""
	directive = strings.TrimSpace(directive)
	for i, r := range directive {
		if !httpguts.IsTokenRune(r) {
			token = directive[:i]
			directive = directive[i:]
			break
		}
	}
	if token == "" {
		token = directive
		directive = ""
	}
	if directive == "" {
		return strings.TrimSpace(token), ""
	}
	if directive[0] != '=' {
		return strings.TrimSpace(token), ""
	}
	directive = directive[1:] // Remove the '='
	if directive == "" {
		return strings.TrimSpace(token), ""
	}
	if directive[0] == '"' {
	}
	for i, r := range directive {
		if r == ',' || r == ';' {
			directive = directive[i:]
			break
		}
		if !httpguts.IsTokenRune(r) {
			return strings.TrimSpace(token), ""
		}
	}
	return strings.TrimSpace(token), strings.TrimSpace(directive)
}

func parseDirectives(directives string) map[string]string {

	result := make(map[string]string)
	for _, directive := range strings.Split(directives, ",") {
		key, value := parseDirective(directive)
		if key != "" {
			result[key] = value
		}
	}
	return result
}

type CacheControl struct {
	Age     *time.Duration
	MaxAge  *time.Duration
	NoCache bool
	NoStore bool
}

func ParseCacheControl(headers http.Header) *CacheControl {
	result := &CacheControl{}
	if age := headers.Get("Age"); age != "" {
		if ageInt, err := strconv.Atoi(age); err == nil {
			age := time.Duration(ageInt) * time.Second
			result.Age = &age
		}
	}
	if cacheControl := headers.Get("Cache-Control"); cacheControl != "" {
		directives := parseDirectives(cacheControl)
		if maxAge, ok := directives["max-age"]; ok {
			if maxAgeInt, err := strconv.Atoi(maxAge); err == nil {
				maxAge := time.Duration(maxAgeInt) * time.Second
				result.MaxAge = &maxAge
			}
		}
		if _, ok := directives["no-cache"]; ok {
			result.NoCache = true
		}
		if _, ok := directives["no-store"]; ok {
			result.NoStore = true
		}
	}
	return result
}
