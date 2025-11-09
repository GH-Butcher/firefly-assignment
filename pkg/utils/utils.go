package utils

import (
	"bufio"

	"errors"
	"io"
	"log/slog"
	"os"

	"strings"
)

// ---> Bank of Words filtering settings
func IsValidWord(word string, minLength int) bool {
	// valid if meets min length and contains ONLY letters (no digits/punct)
	if len(word) < minLength {
		return false
	}
	for i := 0; i < len(word); i++ {
		c := word[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}

func LoadUrlsList(path string, log *slog.Logger) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err = file.Close(); err != nil {
			log.Error(err.Error())
		}
	}()

	var urls []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text())
		// skip empty lines and comments starting with '#'
		if url == "" || strings.HasPrefix(url, "#") {
			continue
		}
		urls = append(urls, url)
	}
	return urls, scanner.Err()
}

func IOReadAllWithLimit(r io.Reader, limit int64) ([]byte, error) {
	var total int64
	buf := make([]byte, 0, 64<<10)
	tmp := make([]byte, 64<<10)
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

func SplitTextToWords(text string) []string {
	var b strings.Builder
	for i := 0; i < len(text); i++ {
		c := text[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			b.WriteByte(c)
		} else {
			b.WriteByte(' ')
		}
	}
	return strings.Fields(b.String())
}

func CountWordsIntoMap(text string, bankOfWords map[string]struct{}, minLength int, dest map[string]int) {
	for _, word := range SplitTextToWords(text) {
		w := strings.ToLower(word)
		if !IsValidWord(w, minLength) {
			continue
		}
		if _, ok := bankOfWords[w]; !ok {
			continue
		}
		dest[w]++
	}
}
