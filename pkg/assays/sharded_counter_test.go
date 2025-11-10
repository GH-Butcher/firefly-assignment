package assays

import (
	"fmt"
	"sync"
	"testing"
)

func TestShardedWordCounter_Basic(t *testing.T) {
	counter := NewShardedWordCounter(4)

	counter.Increment("hello", 1)
	counter.Increment("world", 2)
	counter.Increment("hello", 3)

	result := counter.Flatten()

	if result["hello"] != 4 {
		t.Errorf("Expected hello count 4, got %d", result["hello"])
	}
	if result["world"] != 2 {
		t.Errorf("Expected world count 2, got %d", result["world"])
	}
}

func TestShardedWordCounter_Concurrent(t *testing.T) {
	counter := NewShardedWordCounter(32)
	const numWorkers = 10
	const incrementsPerWorker = 1000

	var wg sync.WaitGroup
	words := []string{"apple", "banana", "cherry", "date", "elderberry"}

	// Simulate 10 workers, each incrementing words 1000 times
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerWorker; j++ {
				for _, word := range words {
					counter.Increment(word, 1)
				}
			}
		}()
	}

	wg.Wait()

	result := counter.Flatten()

	expectedCount := numWorkers * incrementsPerWorker

	for _, word := range words {
		if result[word] != expectedCount {
			t.Errorf("Word %s: expected count %d, got %d", word, expectedCount, result[word])
		}
	}
}

func TestShardedWordCounter_IncrementBatch(t *testing.T) {
	counter := NewShardedWordCounter(16)

	batch1 := map[string]int{
		"apple":  5,
		"banana": 3,
	}
	batch2 := map[string]int{
		"apple":  2,
		"cherry": 7,
	}

	counter.IncrementBatch(batch1)
	counter.IncrementBatch(batch2)

	result := counter.Flatten()

	if result["apple"] != 7 {
		t.Errorf("Expected apple count 7, got %d", result["apple"])
	}
	if result["banana"] != 3 {
		t.Errorf("Expected banana count 3, got %d", result["banana"])
	}
	if result["cherry"] != 7 {
		t.Errorf("Expected cherry count 7, got %d", result["cherry"])
	}
}

func TestShardedWordCounter_ConcurrentBatch(t *testing.T) {
	counter := NewShardedWordCounter(32)
	const numWorkers = 20
	const batchesPerWorker = 100

	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Go(func() {
			for j := 0; j < batchesPerWorker; j++ {
				batch := map[string]int{
					fmt.Sprintf("word_%d", i): 1,
					"common":                  1,
				}
				counter.IncrementBatch(batch)
			}
		})
	}

	wg.Wait()

	result := counter.Flatten()

	// Each worker should have incremented its unique word 100 times
	for i := 0; i < numWorkers; i++ {
		word := fmt.Sprintf("word_%d", i)
		if result[word] != batchesPerWorker {
			t.Errorf("Word %s: expected count %d, got %d", word, batchesPerWorker, result[word])
		}
	}

	// Common word should be incremented by all workers
	expectedCommon := numWorkers * batchesPerWorker
	if result["common"] != expectedCommon {
		t.Errorf("Common word: expected count %d, got %d", expectedCommon, result["common"])
	}
}

func TestShardedWordCounter_HashDistribution(t *testing.T) {
	counter := NewShardedWordCounter(32)

	// Generate 10,000 words
	const numWords = 10000
	words := make([]string, numWords)
	for i := 0; i < numWords; i++ {
		words[i] = fmt.Sprintf("word_%d", i)
	}

	// Track which shard each word goes to
	shardCounts := make([]int, 32)
	for _, word := range words {
		idx := counter.getShardIndex(word)
		shardCounts[idx]++
	}

	// Calculate average and check distribution quality
	average := numWords / 32 // Should be ~312

	// Each shard should have roughly equal words (within 25% of average)
	minExpected := int(float64(average) * 0.75)
	maxExpected := int(float64(average) * 1.25)

	for i, count := range shardCounts {
		if count < minExpected || count > maxExpected {
			t.Errorf("Shard %d has poor distribution: %d words (expected %d±25%%)", i, count, average)
		}
	}
}

func TestShardedWordCounter_Len(t *testing.T) {
	counter := NewShardedWordCounter(8)

	counter.Increment("apple", 5)
	counter.Increment("banana", 3)
	counter.Increment("cherry", 1)

	length := counter.Len()
	if length != 3 {
		t.Errorf("Expected length 3, got %d", length)
	}
}

func TestShardedWordCounter_EmptyCounter(t *testing.T) {
	counter := NewShardedWordCounter(16)

	result := counter.Flatten()
	if len(result) != 0 {
		t.Errorf("Expected empty map, got %d entries", len(result))
	}

	length := counter.Len()
	if length != 0 {
		t.Errorf("Expected length 0, got %d", length)
	}
}

func TestNewShardedWordCounter_PowerOfTwo(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 32},    // Default
		{1, 1},     // Already power of 2
		{2, 2},     // Already power of 2
		{3, 4},     // Round up
		{7, 8},     // Round up
		{15, 16},   // Round up
		{16, 16},   // Already power of 2
		{17, 32},   // Round up
		{31, 32},   // Round up
		{32, 32},   // Already power of 2
		{50, 64},   // Round up
		{100, 128}, // Round up
	}

	for _, tt := range tests {
		counter := NewShardedWordCounter(tt.input)
		actualShards := len(counter.shards)
		if actualShards != tt.expected {
			t.Errorf("NewShardedWordCounter(%d): expected %d shards, got %d", tt.input, tt.expected, actualShards)
		}
	}
}

func TestIsPowerOfTwo(t *testing.T) {
	tests := []struct {
		input    int
		expected bool
	}{
		{0, false},
		{1, true},
		{2, true},
		{3, false},
		{4, true},
		{7, false},
		{8, true},
		{15, false},
		{16, true},
		{32, true},
		{33, false},
		{64, true},
		{128, true},
		{1024, true},
	}

	for _, tt := range tests {
		result := isPowerOfTwo(tt.input)
		if result != tt.expected {
			t.Errorf("isPowerOfTwo(%d): expected %v, got %v", tt.input, tt.expected, result)
		}
	}
}

func TestNextPowerOfTwo(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{5, 8},
		{7, 8},
		{9, 16},
		{15, 16},
		{17, 32},
		{31, 32},
		{33, 64},
		{100, 128},
	}

	for _, tt := range tests {
		result := nextPowerOfTwo(tt.input)
		if result != tt.expected {
			t.Errorf("nextPowerOfTwo(%d): expected %d, got %d", tt.input, tt.expected, result)
		}
	}
}

// Benchmark tests to demonstrate performance improvements

func BenchmarkShardedCounter_Increment(b *testing.B) {
	counter := NewShardedWordCounter(32)
	words := []string{"apple", "banana", "cherry", "date", "elderberry"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		word := words[i%len(words)]
		counter.Increment(word, 1)
	}
}

func BenchmarkShardedCounter_IncrementBatch(b *testing.B) {
	counter := NewShardedWordCounter(32)
	batch := map[string]int{
		"apple":      10,
		"banana":     20,
		"cherry":     15,
		"date":       5,
		"elderberry": 8,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.IncrementBatch(batch)
	}
}

func BenchmarkShardedCounter_ConcurrentIncrement(b *testing.B) {
	counter := NewShardedWordCounter(32)
	words := []string{"apple", "banana", "cherry", "date", "elderberry"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			word := words[i%len(words)]
			counter.Increment(word, 1)
			i++
		}
	})
}

func BenchmarkShardedCounter_Flatten(b *testing.B) {
	counter := NewShardedWordCounter(32)

	// Pre-populate with 10k words
	for i := 0; i < 10000; i++ {
		counter.Increment(fmt.Sprintf("word_%d", i), i%100)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = counter.Flatten()
	}
}
