package style

import (
	"fmt"
	"hash/fnv"
)

// Pick chooses one of count options for a feature, always the same for the same feature ("id"
// property) and purpose (salt), and spread evenly over many features.
func Pick(properties map[string]any, salt string, count int) int {
	if count <= 1 {
		return 0
	}
	return int(Seed(properties, salt) % uint64(count))
}

// Seed is a number that stays the same for a feature and a purpose: for stable random choices.
func Seed(properties map[string]any, salt string) uint64 {
	hash := fnv.New64a()
	fmt.Fprint(hash, properties["id"], "/", salt)
	return hash.Sum64()
}
