package assays

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"firefly-assignment/pkg/models"
)

func ProcessTopWords(ctx context.Context, log *slog.Logger, cfg *Config, bank map[string]struct{}) (Result, error) {
	svc := NewService(log, cfg, bank)
	return svc.ProcessTopWords(ctx)
}

// ProcessTopWords now computes top 10 for EACH assay separately and returns []models.Assay.
func (s *Service) ProcessTopWords(ctx context.Context) (Result, error) {
	// Resolve list path
	listPath := s.cfg.ListPath
	if listPath == "" {
		listPath = "assays.list"
	}

	// Load assay identifiers (names/paths/URLs)
	ids, err := s.loadEssayList(listPath)
	if err != nil {
		return Result{}, err
	}

	// Apply max cap only
	if s.cfg.Max > 0 && s.cfg.Max < len(ids) {
		ids = ids[:s.cfg.Max]
	}
	if len(ids) == 0 {
		return Result{Assays: []models.Assay{}}, nil
	}

	// Concurrency config
	workers := s.cfg.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	buffer := s.cfg.Buffer
	if buffer <= 0 {
		buffer = 128
	}
	rps := s.cfg.RatePerSecond
	if rps <= 0 {
		rps = 20
	}
	minLen := s.cfg.MinWordLength
	if minLen < 1 {
		minLen = 1
	}

	type job struct {
		name string
		id   string
	}
	type result struct {
		assay models.Assay
		err   error
	}

	jobs := make(chan job, buffer)
	results := make(chan result, buffer)

	// Rate limiter (token bucket at RPS)
	tokenCh := make(chan struct{}, rps)
	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(rps))
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case tokenCh <- struct{}{}:
				default:
				}
			}
		}
	}()

	// Progress counter
	var handled atomic.Int64

	// Workers: fetch, count per-assay, emit top 10 for that assay
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				select {
				case <-ctx.Done():
					return
				case <-tokenCh:
				}

				text, e := s.fetchEssay(ctx, j.id)
				if e != nil {
					select {
					case results <- result{err: e}:
					default:
					}
					n := handled.Add(1)
					if n%50 == 0 {
						s.log.Info("Progress", "handled", n, "total", len(ids))
					}
					continue
				}

				// Optional HTML stripping to avoid counting DOM/JS/CSS tokens
				if s.cfg.StripHTML && isHTML(text) {
					text = extractVisibleTextHTML(text)
				}

				// Per-assay count
				counts := make(map[string]int, 1024)
				countWordsIntoMap(text, s.bank, minLen, counts)

				// Top 10 words (strings only)
				top := topN(counts, 10)
				words := make([]string, 0, len(top))
				for _, wc := range top {
					words = append(words, wc.Word)
				}

				assay := models.Assay{
					Name:     j.name,
					TopWords: words,
				}
				results <- result{assay: assay}

				n := handled.Add(1)
				if n%50 == 0 {
					s.log.Info("Progress", "handled", n, "total", len(ids))
				}
			}
		}()
	}

	// Producer: build jobs with name and id.
	// Use the line itself as both name and id; adjust if your list has "name,uri" format.
	go func() {
		defer close(jobs)
		for _, id := range ids {
			name := id
			jobs <- job{name: name, id: id}
		}
	}()

	// Collector
	go func() {
		wg.Wait()
		close(results)
	}()

	var assaysOut []models.Assay
	var firstErr error
	for r := range results {
		if r.err != nil && firstErr == nil {
			firstErr = r.err
			// keep going to collect others
			continue
		}
		if r.assay.Name != "" {
			assaysOut = append(assaysOut, r.assay)
		}
	}

	// Final progress if not on an exact multiple
	if handled.Load()%50 != 0 {
		s.log.Info("Progress", "handled", handled.Load(), "total", len(ids))
	}

	// If nothing succeeded, return the first error
	if len(assaysOut) == 0 && firstErr != nil {
		return Result{}, firstErr
	}

	return Result{Assays: assaysOut}, nil
}

// countWordsIntoMap counts valid words into a provided map (per-assay).
var alphaOnly = regexp.MustCompile(`^[a-zA-Z]+$`)

func countWordsIntoMap(text string, bank map[string]struct{}, minLen int, dst map[string]int) {
	parts := splitWords(text)
	for _, p := range parts {
		if len(p) < minLen {
			continue
		}
		w := strings.ToLower(p)
		if !alphaOnly.MatchString(w) {
			continue
		}
		if _, ok := bank[w]; !ok {
			continue
		}
		dst[w]++
	}
}

// Helpers reused from previous version

func splitWords(s string) []string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			b.WriteByte(c)
		} else {
			b.WriteByte(' ')
		}
	}
	return strings.Fields(b.String())
}

func topN(m map[string]int, n int) []WordCount {
	if n <= 0 {
		return []WordCount{}
	}
	arr := make([]WordCount, 0, len(m))
	for w, c := range m {
		arr = append(arr, WordCount{Word: w, Count: c})
	}
	sort.Slice(arr, func(i, j int) bool {
		if arr[i].Count == arr[j].Count {
			return arr[i].Word < arr[j].Word
		}
		return arr[i].Count > arr[j].Count
	})
	if len(arr) > n {
		arr = arr[:n]
	}
	return arr
}

// HTML detection and stripping helpers

var (
	reHTMLComment   = regexp.MustCompile(`(?s)<!--.*?-->`)
	reScriptBlock   = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)     // remove scripts
	reStyleBlock    = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)       // remove styles
	reNoScriptBlock = regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`) // remove noscript
	reTags          = regexp.MustCompile(`(?s)<[^>]+>`)                        // any remaining tags
	reMultiSpace    = regexp.MustCompile(`\s+`)
)

var htmlEntityReplacer = strings.NewReplacer(
	"&nbsp;", " ",
	"&amp;", "&",
	"&lt;", "<",
	"&gt;", ">",
	"&quot;", "\"",
	"&apos;", "'",
)

func isHTML(s string) bool {
	// Cheap checks: common HTML markers or high tag density
	ls := strings.ToLower(s)
	if strings.Contains(ls, "<html") || strings.Contains(ls, "<!doctype html") || strings.Contains(ls, "<head") || strings.Contains(ls, "<body") {
		return true
	}
	// Heuristic: if there are many angle brackets with tag-like patterns
	if strings.Contains(ls, "<div") || strings.Contains(ls, "<script") || strings.Contains(ls, "<style") || strings.Contains(ls, "<span") || strings.Contains(ls, "<p>") {
		return true
	}
	return false
}

func extractVisibleTextHTML(s string) string {
	// Remove comments and non-visible blocks first
	s = reHTMLComment.ReplaceAllString(s, " ")
	s = reScriptBlock.ReplaceAllString(s, " ")
	s = reStyleBlock.ReplaceAllString(s, " ")
	s = reNoScriptBlock.ReplaceAllString(s, " ")
	// Strip all remaining tags
	s = reTags.ReplaceAllString(s, " ")
	// Decode a few common HTML entities
	s = htmlEntityReplacer.Replace(s)
	// Collapse whitespace
	s = reMultiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// IO and list helpers

func (s *Service) fetchEssay(ctx context.Context, id string) (string, error) {
	if strings.HasPrefix(id, "http://") || strings.HasPrefix(id, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, id, nil)
		if err != nil {
			return "", err
		}
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", errors.New("non-2xx response")
		}
		b, err := ioReadAllLimit(resp.Body, 5<<20) // 5MB cap
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	b, err := os.ReadFile(id)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ioReadAllLimit(r io.Reader, limit int64) ([]byte, error) {
	var total int64
	buf := make([]byte, 0, 64*1024)
	tmp := make([]byte, 64*1024)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			total += int64(n)
			if total > limit {
				return nil, errors.New("response too large")
			}
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return buf, nil
			}
			return nil, err
		}
	}
}

func (s *Service) loadEssayList(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var ids []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		t := strings.TrimSpace(sc.Text())
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		ids = append(ids, t)
	}
	return ids, sc.Err()
}
