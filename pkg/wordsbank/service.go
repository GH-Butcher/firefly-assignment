package wordsbank

import (
	"bufio"
	"context"
	"firefly-assignment/pkg/utils"
	"fmt"
	"net/http"
	"strings"
)

func (c *Config) LoadBankOfWords(ctx context.Context) (map[string]struct{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			c.Log.Error(err.Error())
		}
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)

	bank := make(map[string]struct{}, 64<<10) // 64<<10 = 64K
	for scanner.Scan() {
		word := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if utils.IsValidWord(word, c.MinWordLength) {
			bank[word] = struct{}{}
		}
	}

	return bank, scanner.Err()
}
