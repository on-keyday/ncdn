package cache

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/kota-yata/kyache/cache"
)

/*
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

func parseDirective(directive string) (string, string, string) {
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
		return strings.TrimSpace(token), "", ""
	}
	if directive[0] != '=' {
		return strings.TrimSpace(token), "", directive
	}
	directive = directive[1:] // Remove the '='
	if directive == "" {
		return strings.TrimSpace(token), "", ""
	}
	if directive[0] == '"' {
		directive = directive[1:]
		for i := 0; i < len(directive); i++ {
			if directive[i] == '"' {
				directive = directive[:i]
				return strings.TrimSpace(token), strings.TrimSpace(directive), directive[i+1:]
			}
		}
	} else {
		for i, r := range directive {
			if r == ',' || r == ';' {
				directive = directive[:i]
				return strings.TrimSpace(token), strings.TrimSpace(directive), directive[i+1:]
			}
			if !httpguts.IsTokenRune(r) {
				return strings.TrimSpace(token), "", ""
			}
		}
	}
	return strings.TrimSpace(token), strings.TrimSpace(directive), ""
}

func parseDirectives(directives string) map[string]string {
	result := make(map[string]string)
	data := directives
	for len(data) > 0 {
		key, value, rest := parseDirective(data)
		if key != "" {
			result[key] = value
		}
		data = rest
	}
	return result
}

type ParsedHeader struct {
	Directives map[string]map[string]string
	Values     map[string]string
}

type CacheControl struct {
	Age     *time.Duration
	MaxAge  *time.Duration
	NoCache bool
	NoStore bool
}

func ParseCacheControl(headers http.Header) *CacheControl {
	result := &CacheControl{}
	for key, values := range headers {
		if strings.EqualFold(key, "Cache-Control") {
			for _, value := range values {
				directives := parseDirectives(value)
				if age, ok := directives["max-age"]; ok {
					if d, err := time.ParseDuration(age); err == nil {
						result.MaxAge = &d
					}
				}
				if age, ok := directives["s-maxage"]; ok {
					if d, err := time.ParseDuration(age); err == nil {
						result.MaxAge = &d
					}
				}
				if _, ok := directives["no-cache"]; ok {
					result.NoCache = true
				}
				if _, ok := directives["no-store"]; ok {
					result.NoStore = true
				}
			}
		} else if strings.EqualFold(key, "Age") {
			for _, value := range values {
				if age, err := time.ParseDuration(value); err == nil {
					result.Age = &age
				}
			}
		}
	}
	return result
}
*/

type Cache struct {
	c *cache.CacheStore
}

func NewCache() *Cache {
	return &Cache{
		c: cache.NewCacheStore(),
	}
}

func (c *Cache) GenerateCacheKey(req *http.Request) string {
	reqHeaderStruct := cache.NewParsedHeaders(req.Header)
	return cache.GenerateCacheKey(req.URL.String(), reqHeaderStruct)
}

func (c *Cache) HasCache(r *http.Request) (string, *http.Response, bool) {
	reqHeaderStruct := cache.NewParsedHeaders(r.Header)
	key := c.GenerateCacheKey(r)
	if cached, found := c.c.Get(key); found {
		respHeader := cache.NewParsedHeaders(cached.ResponseHeader)
		originalReqHeaderStruct := cache.NewParsedHeaders(cached.RequestHeader)
		if cache.IsFresh(cached) && cache.IsReqAllowedToUseCache(reqHeaderStruct, originalReqHeaderStruct, respHeader) {
			header := cached.ResponseHeader.Clone()
			header.Set("Age", strconv.Itoa(cache.GetCurrentAge(cached)))
			resp := &http.Response{
				StatusCode:    cached.StatusCode,
				Header:        header,
				Body:          io.NopCloser(bytes.NewReader(cached.Body)),
				ContentLength: int64(len(cached.Body)),
				Request:       r,
				ProtoMajor:    1,
				ProtoMinor:    1,
				Proto:         "HTTP/1.1",
			}
			return key, resp, true
		}
	}
	return key, nil, false
}

type CacheData interface {
	GetValidatedAge() int
}

// currently not depdnent on c but for the future use
func (c *Cache) IsCacheable(resp *http.Response) (CacheData, bool) {
	respHeaderStruct := cache.NewParsedHeaders(resp.Header)
	return respHeaderStruct, cache.IsCacheable(resp.Request.Method, respHeaderStruct)
}

func (c *Cache) SetCache(key string, req *http.Request, resp *http.Response, header CacheData) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}
	resp.Body.Close()                               // Close the original body
	resp.Body = io.NopCloser(bytes.NewReader(body)) // Create a new body reader
	age := header.GetValidatedAge()
	cached := &cache.CachedResponse{
		StatusCode:     resp.StatusCode,
		RequestHeader:  req.Header.Clone(),
		ResponseHeader: resp.Header.Clone(),
		Body:           body,
		StoredAt:       time.Now(),
		InitialAge:     age,
	}
	c.c.Set(key, cached)
	return nil
}
