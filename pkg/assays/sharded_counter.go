package assays

import (
	"hash/fnv"
	"sync"
)

// defaultShardCount is optimized for typical workloads with 8-16 workers.
// Power of 2 enables fast modulo via bitwise AND operation.
const defaultShardCount = 32

// ShardedWordCounter provides thread-safe, high-performance word counting
// by distributing words across multiple shards, each with its own lock.
// This dramatically reduces lock contention compared to a single global mutex.
//
// Performance characteristics:
//   - With N shards and M workers where N > M: ~O(1) lock contention
//   - Memory overhead: ~(N * 64 bytes) for mutex structures + map overhead
//   - Hash operation: ~5-10ns per word (FNV-1a)
//
// Example usage:
//
//	counter := NewShardedWordCounter(32)
//	// In concurrent workers:
//	counter.Increment("apple", 1)
//	counter.Increment("banana", 5)
//	// After all workers complete:
//	finalCounts := counter.Flatten()
type ShardedWordCounter struct {
	shards []*wordShard
	mask   uint32 // For fast modulo: hash & mask instead of hash % len
}

// wordShard represents a single shard containing a subset of words.
// Each shard has its own mutex to allow concurrent access to different shards.
type wordShard struct {
	mu    sync.Mutex
	words map[string]int
}

// NewShardedWordCounter creates a new sharded counter with the specified number of shards.
// shardCount must be a power of 2 for optimal performance. If not, it will be rounded up.
// If shardCount <= 0, defaults to 32.
//
// Recommended shard counts:
//   - 16 shards: 4-8 workers
//   - 32 shards: 8-16 workers (default)
//   - 64 shards: 16-32 workers
//   - 128 shards: 32+ workers
func NewShardedWordCounter(shardCount int) *ShardedWordCounter {
	if shardCount <= 0 {
		shardCount = defaultShardCount
	}
	// Ensure power of 2 for fast modulo via bitwise AND
	if !isPowerOfTwo(shardCount) {
		shardCount = nextPowerOfTwo(shardCount)
	}

	shards := make([]*wordShard, shardCount)
	for i := 0; i < shardCount; i++ {
		shards[i] = &wordShard{
			// Pre-allocate map to reduce allocations during counting
			words: make(map[string]int, 256),
		}
	}

	return &ShardedWordCounter{
		shards: shards,
		mask:   uint32(shardCount - 1), // e.g., 32 shards -> mask = 31 (0b11111)
	}
}

// Increment adds count to the word's total in a thread-safe manner.
// Only locks the specific shard that owns this word, allowing concurrent
// updates to words in different shards.
//
// Time complexity: O(1) average for hash + map lookup
// Lock contention: O(workers/shards) probability of collision
func (sc *ShardedWordCounter) Increment(word string, count int) {
	shard := sc.getShard(word)
	shard.mu.Lock()
	shard.words[word] += count
	shard.mu.Unlock()
}

// IncrementBatch efficiently increments multiple words that are already in a map.
// This is more efficient than calling Increment in a loop because it:
// 1. Sorts words by shard to minimize lock acquisitions
// 2. Processes all words for a shard while holding the lock once
//
// Use this when you have a local word count map to merge into the global counter.
func (sc *ShardedWordCounter) IncrementBatch(words map[string]int) {
	// Create batches for each shard
	batches := make([]map[string]int, len(sc.shards))
	for i := range batches {
		batches[i] = make(map[string]int, len(words)/len(sc.shards)+1)
	}

	// Distribute words to their respective shard batches
	for word, count := range words {
		shardIdx := sc.getShardIndex(word)
		batches[shardIdx][word] = count
	}

	// Update each shard once with all its words
	for i, batch := range batches {
		if len(batch) == 0 {
			continue
		}
		shard := sc.shards[i]
		shard.mu.Lock()
		for word, count := range batch {
			shard.words[word] += count
		}
		shard.mu.Unlock()
	}
}

// getShard determines which shard owns this word using FNV-1a hash.
// FNV-1a is chosen because:
//   - Fast: ~5-10ns per operation
//   - Good distribution: Minimizes collisions
//   - Non-cryptographic: We don't need security, just distribution
//   - Deterministic: Same word always maps to same shard
func (sc *ShardedWordCounter) getShard(word string) *wordShard {
	idx := sc.getShardIndex(word)
	return sc.shards[idx]
}

// getShardIndex returns the shard index for a given word
func (sc *ShardedWordCounter) getShardIndex(word string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(word))
	// Use bitwise AND with mask for fast modulo
	// e.g., hash & 31 is equivalent to hash % 32 but much faster
	return h.Sum32() & sc.mask
}

// Flatten merges all shards into a single map.
// This should only be called after all concurrent writes are complete.
//
// Time complexity: O(total_words)
// Memory: Allocates new map with all words
//
// Note: This is safe to call while workers are still writing, but the result
// may not include words that are currently being processed.
func (sc *ShardedWordCounter) Flatten() map[string]int {
	// Pre-allocate result map with reasonable capacity
	result := make(map[string]int, 4096)

	// Iterate through all shards and merge
	for _, shard := range sc.shards {
		shard.mu.Lock()
		for word, count := range shard.words {
			result[word] += count
		}
		shard.mu.Unlock()
	}

	return result
}

// Len returns the total number of unique words across all shards.
// This requires locking all shards, so use sparingly.
func (sc *ShardedWordCounter) Len() int {
	total := 0
	for _, shard := range sc.shards {
		shard.mu.Lock()
		total += len(shard.words)
		shard.mu.Unlock()
	}
	return total
}

// isPowerOfTwo checks if n is a power of 2
func isPowerOfTwo(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}

// nextPowerOfTwo returns the next power of 2 greater than or equal to n
// Uses bit manipulation for efficiency
func nextPowerOfTwo(n int) int {
	if n <= 0 {
		return 1
	}
	// If already power of 2, return as-is
	if isPowerOfTwo(n) {
		return n
	}
	// Round up to next power of 2
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n++
	return n
}
