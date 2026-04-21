package lsp

type approach = string

var approachEmoji = map[approach]string{
	"rapid":      "🔥",
	"aggressive": "⚡",
	"methodical": "🌍",
	"rigorous":   "💎",
	"analytical": "💧",
	"holistic":   "🌬️",
}

var approachTraits = map[approach]string{
	"rapid":      "Speed: fast\nThoroughness: 1 evidence, 1 loops\nConfidence bar: 0.40\nSkip tolerance: 0.8",
	"aggressive": "Speed: fastest\nThoroughness: 1 evidence, 0 loops\nConfidence bar: 0.30\nSkip tolerance: 0.9",
	"methodical": "Speed: moderate\nThoroughness: 3 evidence, 2 loops\nConfidence bar: 0.60\nSkip tolerance: 0.4",
	"rigorous":   "Speed: slow\nThoroughness: 5 evidence, 3 loops\nConfidence bar: 0.80\nSkip tolerance: 0.1",
	"analytical": "Speed: moderate\nThoroughness: 4 evidence, 2 loops\nConfidence bar: 0.70\nSkip tolerance: 0.3",
	"holistic":   "Speed: variable\nThoroughness: 3 evidence, 2 loops\nConfidence bar: 0.50\nSkip tolerance: 0.5",
}

func lspApproachEmoji(a approach) string {
	return approachEmoji[a]
}

func lspApproachTraitsSummary(a approach) string {
	return approachTraits[a]
}
