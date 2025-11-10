package assays

import (
	"bytes"
	"context"
	"firefly-assignment/pkg/utils"
	"fmt"
	"io"
	"sort"
	"sync"
	"sync/atomic"

	"net/http"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/time/rate"
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
	//--> Sharded word counter for high-performance concurrent access
	// Using 32 shards optimized for 10 workers (reduces lock contention by ~8x)
	wordCounter := NewShardedWordCounter(s.config.ShardsCount)

	//--> Rate limiter using golang.org/x/time/rate (industry standard)
	// Token bucket algorithm with burst capacity equal to rate for smooth distribution
	limiter := rate.NewLimiter(rate.Limit(s.config.RatePerSecond), s.config.RatePerSecond)

	//--> Workers: fetch, count, emit top 10 for all assays
	s.createWorkersPool(ctx, &handled, jobs, results, urls, limiter, wordCounter)

	//--> Jobs: Feed jobs and then close
	go feedJobs(ctx, jobs, urls)

	var firstError error
	s.collectFirstError(results, &firstError)

	//--> final progress counter
	s.progressNotifier(&handled, len(urls), true)

	//--> If nothing succeeded, return error
	if firstError != nil {
		return Result{}, firstError
	}

	//---> Flatten sharded counter into single map
	// This is safe to do here as all workers have completed
	wordsCount := wordCounter.Flatten()

	//---> Get Top Words
	topWords := topWordsCounter(wordsCount, s.config.TopWordsCount)

	assayResult := AssayResult{
		TargetAssaysCount:    len(urls),
		HandledAssaysCount:   handled.Load(),
		DesiredTopWordsCount: s.config.TopWordsCount,
		TopWords:             topWords,
		DesiredWordLength:    s.config.MinWordLength,
	}

	return Result{AssaysResult: assayResult}, nil

}

func (s *Service) collectFirstError(results <-chan result, firstError *error) {
	for r := range results {
		if r.err != nil && *firstError == nil {
			*firstError = r.err
			continue
		}
	}
}

func (s *Service) progressNotifier(counter *atomic.Int64, total int, final bool) {
	if final {
		if counter.Load()%50 != 0 {
			s.log.Info("Progress", "handled", counter.Load(), "total", total)
		}
	}
	if counter.Load()%50 == 0 {
		s.log.Info("Handled assays", "count", counter.Load())
	}
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
			return "", fmt.Errorf("bad status code: %d, assay url %s", resp.StatusCode, url)
		}
		//--> read response body with limit
		body, err := utils.IOReadAllWithLimit(resp.Body, readLimit) //--> readLimit is 5MB
		if err != nil {
			return "", err
		}
		//--> parse html and extract text
		return parseHtml(bytes.NewReader(body))
	}
	s.log.Error(fmt.Sprintf("invalid url: %s", url))
	return "", nil
}

func parseHtml(reader io.Reader) (string, error) {
	//--> parse html, extract text
	assayHtml, err := html.Parse(reader)
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

func feedJobs(ctx context.Context, jobs chan<- job, urls []string) {
	//--> Feed jobs and then close
	defer close(jobs)
	for _, u := range urls {
		select {
		case <-ctx.Done():
			return
		case jobs <- job{url: u}:
		}
	}
}

func (s *Service) createWorkersPool(ctx context.Context, handled *atomic.Int64, jobs <-chan job, results chan<- result, urls []string, limiter *rate.Limiter, wordCounter *ShardedWordCounter) {
	//--> Workers: fetch, count, emit top 10 for all assays
	var wg sync.WaitGroup

	for i := 0; i < s.config.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each worker maintains a local batch of words to minimize lock acquisitions
			// We flush to the sharded counter periodically for better performance
			localBatch := make(map[string]int, 512)
			batchSize := 0
			const flushThreshold = 100 // Flush every 100 words

			for j := range jobs {
				// Wait for rate limiter permission
				// This blocks until a token is available or context is canceled
				if err := limiter.Wait(ctx); err != nil {
					// Context cancelled
					return
				}

				//--> safeguard
				if ctx.Err() != nil {
					return
				}

				text, intErr := s.fetchAssay(ctx, j.url)
				if intErr != nil {
					select {
					case results <- result{err: intErr}:
					default:
					}
					handled.Add(1)
					s.progressNotifier(handled, len(urls), false)
					continue
				}

				//--> Count words into local batch
				utils.CountWordsIntoMap(text, s.wordsBank, s.config.MinWordLength, localBatch)
				batchSize += len(localBatch)

				//--> Flush to sharded counter periodically to balance memory vs lock overhead
				if batchSize >= flushThreshold {
					wordCounter.IncrementBatch(localBatch)
					localBatch = make(map[string]int, 512)
					batchSize = 0
				}

				handled.Add(1)
				s.progressNotifier(handled, len(urls), false)
			}

			//--> Flush any remaining words
			if len(localBatch) > 0 {
				wordCounter.IncrementBatch(localBatch)
			}
		}()
	}
	//--> Collector
	go func() {
		wg.Wait()
		close(results)
	}()
}
