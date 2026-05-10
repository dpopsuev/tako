package cerebrum

import (
	"context"
	"log/slog"
	"math"
)

type IntentResult struct {
	Pipe        *Pipe
	Overlap     float64
	Gear        Gear
	Temperature float64
}

func overlapToTemperature(overlap float64) float64 {
	if overlap >= 0.95 {
		return 0.1
	}
	if overlap <= 0.1 {
		return 5.0
	}
	return 5.0 * math.Exp(-4.0*overlap)
}

func (cb *Cerebrum) classifyIntent(ctx context.Context, need []byte) IntentResult {
	if cb.embedder == nil || cb.reflexStore == nil {
		return IntentResult{Gear: GearNovel}
	}

	embedding, err := cb.embedder.Embed(ctx, string(need))
	if err != nil {
		slog.WarnContext(ctx, "intent.embed_error", slog.Any("error", err))
		return IntentResult{Gear: GearNovel}
	}

	pipe, overlap := cb.reflexStore.Match(embedding)
	gear := selectGear(overlap)

	slog.InfoContext(ctx, "intent.classified",
		slog.Float64("overlap", overlap),
		slog.String("gear", string(gear)))

	if pipe != nil {
		slog.InfoContext(ctx, "intent.match",
			slog.String("pipe", pipe.Name),
			slog.Float64("score", pipe.Score()),
			slog.Int("replays", pipe.Replays))
	}

	temperature := overlapToTemperature(overlap)

	slog.InfoContext(ctx, "intent.temperature",
		slog.Float64("temperature", temperature))

	return IntentResult{Pipe: pipe, Overlap: overlap, Gear: gear, Temperature: temperature}
}
