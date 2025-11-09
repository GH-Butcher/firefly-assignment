package assays

import (
	"bytes"
	"context"
	"errors"
	"firefly-assignment/pkg/utils"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"net/http"
	"strings"

	"golang.org/x/net/html"
)

const readLimit = 5 << 20

type job struct {
	url string
}

type result struct {
	err error
}

func (s *Service) HandleAssays(ctx context.Context) (Result, error) {
	s.log.Info("Handling assays")
	//--> load urls from file
	urls, err := utils.LoadUrlsList(s.config.UrlsListPath, s.log)
	if err != nil {
		return Result{}, err
	}
	//apply filters
	if s.config.MaxUrlsToFetch > 0 && s.config.MaxUrlsToFetch < len(urls) {
		urls = urls[:s.config.MaxUrlsToFetch]
	}
	if len(urls) == 0 {
		return Result{}, nil
	}

	jobs := make(chan job, s.config.Buffer)
	results := make(chan result, s.config.Buffer)

	//--> Progress Counter
	var handled atomic.Int64
	//--> Global words count
	wordsCount := make(map[string]int, 2048)
	var wordsMu sync.Mutex

	//--> Rate limiter (Token bucket at RPS)
	tokenChan := rateLimiter(ctx, s.config.RatePerSecond)

	//--> Workers: fetch, count, emit top 10 for all assays
	var wg sync.WaitGroup
	for i := 0; i < s.config.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				select {
				case <-ctx.Done():
					return
				case <-tokenChan:
				}

				text, intErr := s.fetchAssay(ctx, j.url)
				if intErr != nil {
					select {
					case results <- result{err: intErr}:
					default:
					}
					n := handled.Add(1)
					if n%50 == 0 {
						s.log.Info(fmt.Sprintf("Handled %d assays", n))
					}
					continue
				}
				//--> get words from assay and append to global map
				wordsMu.Lock()
				utils.CountWordsIntoMap(text, s.wordsBank, s.config.MinWordLength, wordsCount)
				wordsMu.Unlock()
				n := handled.Add(1)
				if n%50 == 0 {
					s.log.Info(fmt.Sprintf("Handled %d assays", n))
				}
			}
		}()
	}

	//--> Feed jobs and then close
	go func() {
		defer close(jobs)
		for _, u := range urls {
			select {
			case <-ctx.Done():
				return
			case jobs <- job{url: u}:
			}
		}
	}()

	//--> Collector
	go func() {
		wg.Wait()
		close(results)
	}()

	var firstError error
	for r := range results {
		if r.err != nil && firstError == nil {
			firstError = r.err
			//keep going to collect others
			continue
		}
	}

	//--> final progress counter
	if handled.Load()%50 != 0 {
		s.log.Info("Progress", "handled", handled.Load(), "total", len(urls))
	}

	//--> If nothing succeeded, return error
	if firstError != nil && len(wordsCount) == 0 {
		return Result{}, firstError
	}

	//---> Get Top Words
	topWords := topWordsCounter(wordsCount, s.config.TopWordsCount)

	assayResult := AssayResult{
		AssaysCount:          len(urls),
		DesiredTopWordsCount: s.config.TopWordsCount,
		TopWords:             topWords,
	}

	return Result{AssaysResult: assayResult}, nil

}

func rateLimiter(ctx context.Context, ratePerSecond int) chan struct{} {
	tokenChan := make(chan struct{}, ratePerSecond)

	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(ratePerSecond))
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case tokenChan <- struct{}{}:
				default:
				}
			}
		}
	}()

	return tokenChan
}

// fetchAssay fetches the assay from the given url, parses it and returns the text
func (s *Service) fetchAssay(ctx context.Context, url string) (string, error) {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return "", err
		}
		defer func() {
			if err = resp.Body.Close(); err != nil {
				s.log.Error(err.Error())
			}
		}()

		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			if resp.StatusCode == http.StatusNotFound {
				return "", nil
			}
			return "", errors.New(fmt.Sprintf("bad status code: %d, assay url %s", resp.StatusCode, url))
		}
		//--> read response body with limit
		body, err := utils.IOReadAllWithLimit(resp.Body, readLimit) //--> readLimit is 5MB
		if err != nil {
			return "", err
		}
		//--> parse html, extract text
		assayHtml, err := html.Parse(bytes.NewReader(body))
		if err != nil {
			return "", err
		}

		var sb strings.Builder
		//--> recursive function to walk nodes
		var walk func(*html.Node, bool)

		walk = func(n *html.Node, skip bool) {
			if n.Type == html.ElementNode {
				// tags to skip entirely (and their children)
				switch n.Data {
				case "script", "style", "noscript", "iframe", "header", "footer", "nav", "aside":
					skip = true
				}
			}
			if !skip {
				if n.Type == html.TextNode {
					text := strings.TrimSpace(n.Data)
					if len(text) > 0 {
						sb.WriteString(text)
						sb.WriteString(" ")
					}
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c, skip)
				}
			}
		}
		walk(assayHtml, false)

		return sb.String(), nil
	}
	s.log.Error(fmt.Sprintf("invalid url: %s", url))
	return "", nil
}

func topWordsCounter(words map[string]int, desiredTopN int) []WordCount {
	if desiredTopN <= 0 {
		return []WordCount{}
	}
	topWords := make([]WordCount, 0, len(words))
	for k, v := range words {
		topWords = append(topWords, WordCount{Word: k, Count: v})
	}
	sort.Slice(topWords, func(i, j int) bool {
		if topWords[i].Count == topWords[j].Count {
			return topWords[i].Word < topWords[j].Word
		}
		return topWords[i].Count > topWords[j].Count
	})
	if len(topWords) > desiredTopN {
		return topWords[:desiredTopN]
	}
	return topWords
}
