package rag

import "strings"

type ChunkConfig struct {
	MaxTokens  int
	OverlapPct int
	Strategy   string
}

type Chunk struct {
	Content string
	Index   int
}

func DefaultChunkConfig(tokens, overlap int) ChunkConfig {
	if tokens <= 0 {
		tokens = 400
	}
	if overlap <= 0 || overlap >= 100 {
		overlap = 15
	}
	return ChunkConfig{
		MaxTokens:  tokens,
		OverlapPct: overlap,
		Strategy:   "sentence",
	}
}

func ChunkText(text string, cfg ChunkConfig) []Chunk {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	maxWords := cfg.MaxTokens
	if maxWords <= 0 {
		maxWords = 400
	}
	step := maxWords - (maxWords*cfg.OverlapPct)/100
	if step <= 0 {
		step = maxWords
	}
	out := make([]Chunk, 0, len(words)/step+1)
	idx := 0
	for start := 0; start < len(words); start += step {
		end := start + maxWords
		if end > len(words) {
			end = len(words)
		}
		if end <= start {
			break
		}
		out = append(out, Chunk{
			Content: strings.Join(words[start:end], " "),
			Index:   idx,
		})
		idx++
		if end == len(words) {
			break
		}
	}
	return out
}
