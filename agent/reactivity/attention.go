package reactivity

import (
	"math"
	"sort"
	"time"
)

type WeightedAtom struct {
	Atom  *Atom
	Score float64
}

func (m *Molecule) Attend(query []float64, temperature float64) []WeightedAtom {
	m.mu.RLock()
	atoms := make([]*Atom, 0, len(m.atoms))
	for _, a := range m.atoms {
		atoms = append(atoms, a)
	}
	residual := m.Residual()
	m.mu.RUnlock()

	if len(atoms) == 0 {
		return nil
	}
	if temperature <= 0 {
		temperature = 1.0
	}

	scores := make([]float64, len(atoms))
	for i, atom := range atoms {
		scores[i] = multiHeadScore(query, atom, m, residual)
	}

	weights := softmaxTemperature(scores, temperature)

	result := make([]WeightedAtom, len(atoms))
	for i := range atoms {
		result[i] = WeightedAtom{Atom: atoms[i], Score: weights[i]}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Score > result[j].Score })
	return result
}

const (
	weightSemantic     = 0.4
	weightStructural   = 0.2
	weightTemporal     = 0.2
	weightDimensional  = 0.2
)

func multiHeadScore(query []float64, atom *Atom, m *Molecule, residual map[string]float64) float64 {
	var score float64
	score += weightSemantic * semanticHead(query, atom)
	score += weightStructural * structuralHead(atom, m)
	score += weightTemporal * temporalHead(atom)
	score += weightDimensional * dimensionalHead(atom, residual)
	return score
}

func semanticHead(query []float64, atom *Atom) float64 {
	if len(query) == 0 || len(atom.Embedding) == 0 {
		return 0
	}
	return cosineSim(query, atom.Embedding)
}

func structuralHead(atom *Atom, m *Molecule) float64 {
	edges := m.edgeIndex[atom.ID]
	if len(edges) > 0 {
		return 1.0
	}
	for _, indices := range m.edgeIndex {
		for _, idx := range indices {
			if m.edges[idx].To == atom.ID {
				return 0.8
			}
		}
	}
	return 0.0
}

func temporalHead(atom *Atom) float64 {
	if atom.CreatedAt.IsZero() {
		return 0.5
	}
	age := time.Since(atom.CreatedAt).Seconds()
	return math.Exp(-age / 300.0)
}

func dimensionalHead(atom *Atom, residual map[string]float64) float64 {
	if len(residual) == 0 || len(atom.Dimensions) == 0 {
		return 0
	}
	hits := 0
	for _, dim := range atom.Dimensions {
		if gap, ok := residual[dim]; ok && gap > 0 {
			hits++
		}
	}
	return float64(hits) / float64(len(atom.Dimensions))
}

func softmaxTemperature(scores []float64, temperature float64) []float64 {
	if len(scores) == 0 {
		return nil
	}

	maxScore := scores[0]
	for _, s := range scores[1:] {
		if s > maxScore {
			maxScore = s
		}
	}

	exps := make([]float64, len(scores))
	var sum float64
	for i, s := range scores {
		exps[i] = math.Exp((s - maxScore) / temperature)
		sum += exps[i]
	}

	if sum == 0 {
		uniform := 1.0 / float64(len(scores))
		for i := range exps {
			exps[i] = uniform
		}
		return exps
	}

	for i := range exps {
		exps[i] /= sum
	}
	return exps
}

func cosineSim(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}
