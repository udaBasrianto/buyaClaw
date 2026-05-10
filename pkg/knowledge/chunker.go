package knowledge

import (
	"strings"
	"unicode"
)

const (
	DefaultChunkSize    = 512  // target tokens per chunk
	DefaultChunkOverlap = 64   // overlap tokens between chunks
	avgCharsPerToken    = 4    // rough estimate: 1 token ≈ 4 chars
)

// ChunkConfig controls how text is split into chunks.
type ChunkConfig struct {
	ChunkSize    int // target token count per chunk
	ChunkOverlap int // token overlap between consecutive chunks
}

// DefaultChunkConfig returns sensible defaults.
func DefaultChunkConfig() ChunkConfig {
	return ChunkConfig{
		ChunkSize:    DefaultChunkSize,
		ChunkOverlap: DefaultChunkOverlap,
	}
}

// ChunkText splits text into overlapping chunks suitable for indexing.
// Splitting prefers paragraph and sentence boundaries over hard character cuts.
func ChunkText(text string, cfg ChunkConfig) []string {
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = DefaultChunkSize
	}
	if cfg.ChunkOverlap < 0 {
		cfg.ChunkOverlap = 0
	}
	if cfg.ChunkOverlap >= cfg.ChunkSize {
		cfg.ChunkOverlap = cfg.ChunkSize / 4
	}

	// Convert token counts to approximate character counts
	chunkChars := cfg.ChunkSize * avgCharsPerToken
	overlapChars := cfg.ChunkOverlap * avgCharsPerToken

	// Normalize whitespace
	text = normalizeText(text)
	if len(text) == 0 {
		return nil
	}

	// Split into paragraphs first
	paragraphs := splitParagraphs(text)

	var chunks []string
	var current strings.Builder

	flush := func() {
		s := strings.TrimSpace(current.String())
		if s != "" {
			chunks = append(chunks, s)
		}
		current.Reset()
	}

	for _, para := range paragraphs {
		if len(para) == 0 {
			continue
		}

		// If adding this paragraph would exceed chunk size, flush first
		if current.Len()+len(para)+1 > chunkChars && current.Len() > 0 {
			flush()

			// Add overlap from end of previous chunk
			if overlapChars > 0 && len(chunks) > 0 {
				prev := chunks[len(chunks)-1]
				if len(prev) > overlapChars {
					overlap := prev[len(prev)-overlapChars:]
					// Find a clean word boundary
					if idx := strings.IndexFunc(overlap, unicode.IsSpace); idx >= 0 {
						overlap = overlap[idx+1:]
					}
					current.WriteString(overlap)
					current.WriteString(" ")
				}
			}
		}

		// If a single paragraph is larger than chunk size, split by sentences
		if len(para) > chunkChars {
			sentences := splitSentences(para)
			for _, sent := range sentences {
				if current.Len()+len(sent)+1 > chunkChars && current.Len() > 0 {
					flush()
					if overlapChars > 0 && len(chunks) > 0 {
						prev := chunks[len(chunks)-1]
						if len(prev) > overlapChars {
							overlap := prev[len(prev)-overlapChars:]
							if idx := strings.IndexFunc(overlap, unicode.IsSpace); idx >= 0 {
								overlap = overlap[idx+1:]
							}
							current.WriteString(overlap)
							current.WriteString(" ")
						}
					}
				}
				current.WriteString(sent)
				current.WriteString(" ")
			}
		} else {
			current.WriteString(para)
			current.WriteString("\n\n")
		}
	}

	flush()
	return chunks
}

// estimateTokens returns a rough token count for a string.
func estimateTokens(s string) int {
	return (len(s) + avgCharsPerToken - 1) / avgCharsPerToken
}

// normalizeText cleans up whitespace in text.
func normalizeText(text string) string {
	// Replace Windows line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Collapse runs of 3+ newlines to 2
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(text)
}

// splitParagraphs splits text on blank lines.
func splitParagraphs(text string) []string {
	parts := strings.Split(text, "\n\n")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// splitSentences splits text on sentence-ending punctuation.
func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	runes := []rune(text)
	for i, r := range runes {
		current.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			// Check next char is space or end of string
			if i+1 >= len(runes) || unicode.IsSpace(runes[i+1]) {
				s := strings.TrimSpace(current.String())
				if s != "" {
					sentences = append(sentences, s)
				}
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		s := strings.TrimSpace(current.String())
		if s != "" {
			sentences = append(sentences, s)
		}
	}
	return sentences
}
