package restclient

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

func Parse(content string) (*RestClient, error) {
	ret, err := parse(content)
	if err != nil {
		return nil, fmt.Errorf("restClient parse error: %w", err)
	}

	ret.Transport = &http.Transport{
		Proxy:              http.ProxyFromEnvironment,
		DisableCompression: true,
	}
	return ret, err
}

type RestClient struct {
	*http.Transport             // Embed the default HTTP round tripper
	method          string      // HTTP method, e.g., "GET", "POST"
	path            string      // Request path, e.g., "/api/data"
	queryParams     url.Values  // Query parameters, e.g., "key=value&key2=value2"
	version         string      // HTTP version, e.g., "HTTP/1.1"
	header          http.Header // HTTP headers
	contentLines    []string
	result          *RestResult
}

func (rc *RestClient) RoundTrip(req *http.Request) (*http.Response, error) {
	rsp, err := rc.Transport.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("restClient RoundTrip error: %w", err)
	}

	rr := &RestResult{}
	if err := rr.Load(rsp); err != nil {
		return nil, err
	}
	rc.result = rr
	return rsp, nil
}

// parse parses the content string into a RestClient.
// e.g. < file.txt
// e.g. <@latin1 file.txt  <- <@charset is not implemented yet
var restClientFileRegexp = regexp.MustCompile(`<(?:@([\w\-]+))?\s+([^\s]+)`)

func (rc *RestClient) Do() *RestResult {
	var client = &http.Client{Transport: rc}
	var payload io.Reader
	if rc.contentLines != nil && len(rc.contentLines) > 0 {
		contentType := rc.header.Get("Content-Type")
		if contentType == "application/x-www-form-urlencoded" {
			b := &strings.Builder{}
			for i, line := range rc.contentLines {
				if i == 0 || strings.HasPrefix(line, "&") {
					b.WriteString(line)
				} else {
					b.WriteString(line + "\n")
				}
			}
			payload = strings.NewReader(b.String())
		} else if strings.Contains(contentType, "multipart/form-data") {
			readers := make([]io.Reader, len(rc.contentLines))
			for i, line := range rc.contentLines {
				r, err := rc.paseFileLine(line)
				if err != nil {
					readers[i] = strings.NewReader(fmt.Sprintf("Error opening file: %v", err))
					continue
				}
				if r == nil {
					readers[i] = strings.NewReader(line + "\n")
					continue
				}
				if closer, ok := r.(io.Closer); ok {
					defer closer.Close()
				}
				readers[i] = r
			}
			payload = io.MultiReader(readers...)
		} else {
			if r, err := rc.paseFileLine(rc.contentLines[0]); err != nil {
				payload = strings.NewReader(fmt.Sprintf("Error opening file: %v", err))
			} else if r != nil {
				readers := make([]io.Reader, len(rc.contentLines))
				if closer, ok := r.(io.Closer); ok {
					defer closer.Close()
				}
				readers[0] = r
				for i, line := range rc.contentLines[1:] {
					r, err := rc.paseFileLine(line)
					if err != nil {
						readers[i+1] = strings.NewReader(fmt.Sprintf("Error opening file: %v", err))
						continue
					}
					if closer, ok := r.(io.Closer); ok {
						defer closer.Close()
					}
					readers = append(readers, r)
				}
				payload = io.MultiReader(readers...)
			} else {
				payload = strings.NewReader(strings.Join(rc.contentLines, "\n"))
			}
		}
	}

	req, err := http.NewRequest(rc.method, rc.path, payload)
	if err != nil {
		return &RestResult{Err: err}
	}
	req.Header = rc.header
	if rc.version != "" {
		// This does not affect the HTTP version used by the client.
		// The HTTP client code always uses either HTTP/1.1 or HTTP/2.
		req.Proto = rc.version
	}

	rsp, err := client.Do(req)
	if err != nil {
		return &RestResult{Err: err}
	}
	defer rsp.Body.Close()

	return rc.result
}

func (rc *RestClient) paseFileLine(line string) (io.Reader, error) {
	if slices.Contains([]string{"POST", "PUT", "PATCH"}, strings.ToUpper(rc.method)) {
		match := restClientFileRegexp.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) != 3 {
			return nil, nil
		}
	}
	var ret io.Reader = nil
	if strings.HasPrefix(line, "< ") || strings.HasPrefix(line, "<@") {
		match := restClientFileRegexp.FindStringSubmatch(line)
		if len(match) != 3 {
			ret = strings.NewReader(line + "\n")
		} else { // match[1] charset, or empty
			var fileLoader FileLoader
			var path = match[2]
			if strings.HasPrefix(path, "@") {
				path = path[1:]
				fileLoader = OS_FileLoader
			} else {
				fileLoader = SSFS_FileLoader
			}
			in, e := fileLoader(path) // file path
			if e != nil {
				return nil, e
			}
			ret = io.MultiReader(in, strings.NewReader("\n"))
		}
	} else {
		ret = strings.NewReader(line + "\n")
	}
	return ret, nil
}

type FileLoader func(string) (io.ReadCloser, error)

func OS_FileLoader(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func SSFS_FileLoader(path string) (io.ReadCloser, error) {
	def := ssfs.Default()
	ent, err := def.Get(path)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewBuffer(ent.Content)), nil
}

type RestResult struct {
	StatusLine      string `json:"statusLine"`
	Header          Header `json:"header"`
	Body            *Body  `json:"body,omitempty"`
	ContentType     string `json:"contentType,omitempty"`
	ContentEncoding string `json:"contentEncoding,omitempty"`
	Err             error  `json:"error,omitempty"`
	dumpString      string `json:"-"`
}

func (rr *RestResult) String() string {
	if rr.Err != nil {
		return rr.Err.Error()
	}
	if rr.dumpString == "" {
		w := &strings.Builder{}
		// Status line
		if _, err := fmt.Fprintf(w, "%s\r\n", rr.StatusLine); err != nil {
			return err.Error()
		}
		// Headers
		for _, h := range rr.Header {
			if _, err := fmt.Fprintf(w, "%s: %s\r\n", h.Name, h.Value); err != nil {
				return err.Error()
			}
		}
		// End-of-header
		if _, err := io.WriteString(w, "\r\n"); err != nil {
			return err.Error()
		}
		// Body
		if rr.Body != nil {
			if _, err := fmt.Fprintf(w, "%s", rr.Body.String()); err != nil {
				return err.Error()
			}
		}
		rr.dumpString = w.String()
	}
	return rr.dumpString
}

func (rr *RestResult) Load(r *http.Response) error {
	// Get content type without charset, if any
	if ct := r.Header.Get("Content-Type"); ct != "" {
		parts := strings.SplitN(ct, ";", 2)
		if len(parts) > 0 {
			rr.ContentType = strings.TrimSpace(parts[0])
		}
	}
	rr.ContentEncoding = r.Header.Get("Content-Encoding")

	if err := rr.loadStatusLine(r); err != nil {
		return fmt.Errorf("error loading status line: %w", err)
	}
	if err := rr.loadHeader(r); err != nil {
		return fmt.Errorf("error loading header: %w", err)
	}
	if err := rr.loadBody(r); err != nil {
		return fmt.Errorf("error dumping response body: %w", err)
	}
	return nil
}

func (rr *RestResult) loadStatusLine(r *http.Response) error {
	// Status line
	text := r.Status
	if text == "" {
		text = http.StatusText(r.StatusCode)
		if text == "" {
			text = "status code " + strconv.Itoa(r.StatusCode)
		}
	} else {
		// Just to reduce stutter, if user set r.Status to "200 OK" and StatusCode to 200.
		// Not important.
		text = strings.TrimPrefix(text, strconv.Itoa(r.StatusCode)+" ")
	}

	rr.StatusLine = fmt.Sprintf("HTTP/%d.%d %03d %s", r.ProtoMajor, r.ProtoMinor, r.StatusCode, text)
	return nil
}

func (rr *RestResult) loadHeader(r *http.Response) error {
	// Header
	keys := []string{}
	for k := range r.Header {
		keys = append(keys, k)
	}
	// Sort keys for consistent output
	slices.Sort(keys)
	// each header line
	for _, k := range keys {
		for _, v := range r.Header.Values(k) {
			rr.Header = append(rr.Header, NameValue{Name: k, Value: v})
		}
	}
	return nil
}

func (rr *RestResult) loadBody(r *http.Response) error {
	if len(rr.ContentType) == 0 {
		return nil
	}

	rr.Body = &Body{
		ContentType:     rr.ContentType,
		ContentEncoding: rr.ContentEncoding,
	}

	out := &bytes.Buffer{}
	_, err := io.Copy(out, r.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}
	rr.Body.Content = out.Bytes()

	return nil
}

type NameValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (h *NameValue) String() string {
	return fmt.Sprintf("%s: %s", h.Name, h.Value)
}

type Header []NameValue

func (h Header) String() string {
	var sb strings.Builder
	for i, header := range h {
		if i > 0 {
			sb.WriteString("\r\n")
		}
		sb.WriteString(header.String())
	}
	return sb.String()
}

type Body struct {
	ContentType     string `json:"contentType,omitempty"`
	ContentEncoding string `json:"contentEncoding,omitempty"`
	Content         []byte `json:"content,omitempty"`
	contentString   string `json:"-"`
}

var printableContentTypes = []string{
	"text/*",
	"application/json",
	"application/javascript",
	"application/x-ndjson",
	"application/xml",
	"application/xhtml+xml",
	"application/x-www-form-urlencoded",
	"application/atom+xml",
	"application/rss+xml",
	"application/geo+json",
	"application/hal+json",
	"application/hal+xml",
	"application/ld+json",
	"application/vnd.api+json",
	"application/vnd.collection+json",
	"application/vnd.geo+json",
}

func isPrintableContentType(contentType string) bool {
	if contentType == "" {
		return false
	}
	for _, ct := range printableContentTypes {
		if strings.HasSuffix(ct, "/*") {
			ct = strings.TrimSuffix(ct, "/*")
			if strings.HasPrefix(contentType, ct) {
				return true
			}
		} else if contentType == ct {
			return true
		}
	}
	return false
}

func (b *Body) String() string {
	if b == nil || len(b.Content) == 0 {
		return ""
	}
	if len(b.contentString) > 0 {
		return b.contentString
	}
	var in io.Reader = bytes.NewReader(b.Content)
	if b.ContentEncoding == "gzip" {
		if r, err := gzip.NewReader(in); err != nil {
			b.contentString = fmt.Sprintf("gzip error: %s", err.Error())
			return b.contentString
		} else {
			defer r.Close()
			in = r
		}
	}
	var out = &strings.Builder{}
	if b.ContentType == "application/json" {
		dec := json.NewDecoder(in)
		var m any
		if err := dec.Decode(&m); err == nil {
			// If the body is valid JSON, we can pretty-print it.
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			if err := enc.Encode(m); err != nil {
				b.contentString = fmt.Sprintf("error encoding JSON: %s", err.Error())
			} else {
				b.contentString = out.String()
			}
		} else {
			// If the body is not valid JSON, we just dump it as is.
			b.contentString = "Invalid JSON: " + err.Error()
		}
	} else if isPrintableContentType(b.ContentType) {
		_, err := io.Copy(out, in)
		if err != nil {
			b.contentString = fmt.Sprintf("error reading body: %s", err.Error())
		} else {
			b.contentString = out.String()
		}
	} else {
		d := hex.Dumper(out)
		if _, err := io.Copy(d, in); err != nil {
			b.contentString = fmt.Sprintf("error dumping body: %s", err.Error())
		} else {
			d.Close()
			b.contentString = out.String()
		}
	}
	return b.contentString
}
