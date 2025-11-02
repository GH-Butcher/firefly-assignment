package words_bank

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

var lettersOnly = regexp.MustCompile(`^[A-Za-z]+$`)

func isValidWord(word string, minLen int) bool {
	return len(word) >= minLen && lettersOnly.MatchString(word)
}

func LoadWordBank(ctx context.Context, config *Config) (map[string]struct{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, config.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := config.Client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("word bank http status %d", resp.StatusCode)
	}

	sc := bufio.NewScanner(resp.Body)

	bank := make(map[string]struct{}, 64_000)
	for sc.Scan() {
		w := strings.ToLower(strings.TrimSpace(sc.Text()))
		if isValidWord(w, config.MinWordLength) {
			bank[w] = struct{}{}
		}
	}
	if err = sc.Err(); err != nil {
		return nil, err
	}
	return bank, nil
}
