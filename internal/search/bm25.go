package search

import (
	"math"
	"sort"
)

// bm25Index is the immutable lexical index over the formatter-v1 tool text.
type bm25Index struct {
	texts             [][]string
	lengths           []float64
	names             []string
	average           float64
	documentFrequency map[string]int
}

func newBM25Index(documents []Document) *bm25Index {
	index := &bm25Index{
		texts:             make([][]string, len(documents)),
		lengths:           make([]float64, len(documents)),
		names:             make([]string, len(documents)),
		documentFrequency: make(map[string]int, len(documents)*8),
	}
	total := 0
	for i, document := range documents {
		index.names[i] = document.PublicName
		tokens := tokenize(document.text())
		index.texts[i] = tokens
		index.lengths[i] = float64(len(tokens))
		total += len(tokens)
		seen := make(map[string]bool, len(tokens))
		for _, token := range tokens {
			if seen[token] {
				continue
			}
			seen[token] = true
			index.documentFrequency[token]++
		}
	}
	if len(documents) > 0 {
		index.average = float64(total) / float64(len(documents))
	}
	return index
}

// rank returns the document indices with a positive BM25 score, best first.
// Ties break on the stable public name.
func (ix *bm25Index) rank(query []string, k1, b float64) []int {
	if len(ix.texts) == 0 || len(query) == 0 {
		return nil
	}
	scores := make([]float64, len(ix.texts))
	for _, term := range query {
		documentFrequency := ix.documentFrequency[term]
		if documentFrequency == 0 {
			continue
		}
		idf := math.Log(1 + (float64(len(ix.texts))-float64(documentFrequency)+0.5)/(float64(documentFrequency)+0.5))
		for i, tokens := range ix.texts {
			termFrequency := 0
			for _, token := range tokens {
				if token == term {
					termFrequency++
				}
			}
			if termFrequency == 0 {
				continue
			}
			normalizer := float64(termFrequency) + k1*(1-b+b*ix.lengths[i]/ix.average)
			scores[i] += idf * float64(termFrequency) * (k1 + 1) / normalizer
		}
	}
	return ix.topIndices(scores)
}

// topIndices returns the positively-scored indices ordered by score descending
// with a stable public-name tie break.
func (ix *bm25Index) topIndices(scores []float64) []int {
	ranked := make([]int, 0, len(scores))
	for i, score := range scores {
		if score > 0 {
			ranked = append(ranked, i)
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if scores[ranked[i]] != scores[ranked[j]] {
			return scores[ranked[i]] > scores[ranked[j]]
		}
		return ix.names[ranked[i]] < ix.names[ranked[j]]
	})
	return ranked
}
