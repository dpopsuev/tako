package mcp

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// heraldic adjectives — colors and qualities from coats of arms.
var heraldicAdjectives = []string{
	"crimson", "azure", "golden", "argent", "sable",
	"verdant", "scarlet", "cobalt", "amber", "ivory",
	"obsidian", "copper", "iron", "silver", "bronze",
	"emerald", "ruby", "sapphire", "onyx", "pearl",
}

// heraldic animals — charges from coats of arms.
var heraldicAnimals = []string{
	"griffin", "falcon", "lion", "stag", "wolf",
	"eagle", "dragon", "phoenix", "raven", "serpent",
	"bear", "hawk", "stallion", "leopard", "boar",
	"heron", "owl", "lynx", "fox", "crane",
}

// GenerateHeraldicName produces a random adjective-animal name.
// 400 unique combinations (20 × 20).
func GenerateHeraldicName() string {
	adj := heraldicAdjectives[cryptoRandN(len(heraldicAdjectives))]
	animal := heraldicAnimals[cryptoRandN(len(heraldicAnimals))]
	return fmt.Sprintf("%s-%s", adj, animal)
}

func cryptoRandN(n int) int {
	bound := big.NewInt(int64(n))
	v, _ := rand.Int(rand.Reader, bound)
	return int(v.Int64())
}
