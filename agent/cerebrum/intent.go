package cerebrum

import (
	"context"
	"log/slog"
)

type IntentResult struct {
	Pipe    *Pipe
	Overlap float64
	Gear    Gear
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

	return IntentResult{Pipe: pipe, Overlap: overlap, Gear: gear}
}
