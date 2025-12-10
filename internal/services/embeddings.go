package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	"github.com/tedkulp/tix/internal/logger"
)

// Chunk represents a segment of a diff with metadata
type Chunk struct {
	Content  string
	Index    int
	FilePath string
	Lines    int
}

// EmbeddingVector represents a chunk with its embedding vector
type EmbeddingVector struct {
	Chunk     *Chunk
	Embedding []float32
}

// VectorStore holds embeddings and provides similarity search
type VectorStore struct {
	Vectors []EmbeddingVector
}

// ChunkDiff splits a diff into manageable chunks
// Each chunk is ~500-1000 lines or represents a complete file if smaller
func ChunkDiff(diff string) []Chunk {
	var chunks []Chunk
	lines := strings.Split(diff, "\n")

	const maxLinesPerChunk = 800
	const minLinesPerChunk = 100

	// Try to split by files first
	var currentChunk strings.Builder
	var currentFilePath string
	var currentLines int
	chunkIndex := 0

	for i, line := range lines {
		// Detect file headers in diff format
		if strings.HasPrefix(line, "diff --git") || strings.HasPrefix(line, "+++") {
			// Extract file path
			if strings.HasPrefix(line, "+++ b/") {
				currentFilePath = strings.TrimPrefix(line, "+++ b/")
			}
		}

		currentChunk.WriteString(line)
		currentChunk.WriteString("\n")
		currentLines++

		// Create a chunk when we hit the max size or at file boundaries
		shouldCreateChunk := false
		if currentLines >= maxLinesPerChunk {
			shouldCreateChunk = true
		} else if i < len(lines)-1 && strings.HasPrefix(lines[i+1], "diff --git") && currentLines >= minLinesPerChunk {
			// New file starting and we have minimum content
			shouldCreateChunk = true
		}

		if shouldCreateChunk {
			content := currentChunk.String()
			if strings.TrimSpace(content) != "" {
				chunks = append(chunks, Chunk{
					Content:  content,
					Index:    chunkIndex,
					FilePath: currentFilePath,
					Lines:    currentLines,
				})
				chunkIndex++
			}
			currentChunk.Reset()
			currentLines = 0
		}
	}

	// Add remaining content
	if currentChunk.Len() > 0 {
		content := currentChunk.String()
		if strings.TrimSpace(content) != "" {
			chunks = append(chunks, Chunk{
				Content:  content,
				Index:    chunkIndex,
				FilePath: currentFilePath,
				Lines:    currentLines,
			})
		}
	}

	logger.Debug("Chunked diff", map[string]any{
		"total_chunks": len(chunks),
		"total_lines":  len(lines),
	})

	return chunks
}

// GenerateEmbeddings creates embeddings for all chunks using OpenAI's API
func GenerateEmbeddings(ctx context.Context, client *openai.Client, chunks []Chunk) ([]EmbeddingVector, error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks to embed")
	}

	vectors := make([]EmbeddingVector, 0, len(chunks))

	// Process in batches to avoid rate limits (OpenAI allows up to 2048 inputs per request)
	batchSize := 50
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[i:end]
		inputs := make([]string, len(batch))
		for j, chunk := range batch {
			inputs[j] = chunk.Content
		}

		logger.Debug("Generating embeddings", map[string]any{
			"batch_start": i,
			"batch_size":  len(batch),
		})

		resp, err := client.CreateEmbeddings(ctx, openai.EmbeddingRequestStrings{
			Input: inputs,
			Model: openai.SmallEmbedding3,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to generate embeddings: %w", err)
		}

		for j, embeddingData := range resp.Data {
			vectors = append(vectors, EmbeddingVector{
				Chunk:     &batch[j],
				Embedding: embeddingData.Embedding,
			})
		}
	}

	logger.Info("Generated embeddings", map[string]any{
		"total_vectors": len(vectors),
	})

	return vectors, nil
}

// CosineSimilarity calculates the cosine similarity between two vectors
func CosineSimilarity(v1, v2 []float32) float64 {
	if len(v1) != len(v2) {
		return 0.0
	}

	var dotProduct, norm1, norm2 float64
	for i := range v1 {
		dotProduct += float64(v1[i]) * float64(v2[i])
		norm1 += float64(v1[i]) * float64(v1[i])
		norm2 += float64(v2[i]) * float64(v2[i])
	}

	if norm1 == 0 || norm2 == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
}

// NewVectorStore creates a new vector store with the given embeddings
func NewVectorStore(vectors []EmbeddingVector) *VectorStore {
	return &VectorStore{
		Vectors: vectors,
	}
}

// SearchResult represents a search result with similarity score
type SearchResult struct {
	Vector     *EmbeddingVector
	Similarity float64
}

// Search finds the top-K most similar chunks to the query embedding
func (vs *VectorStore) Search(queryEmbedding []float32, topK int) []SearchResult {
	if len(vs.Vectors) == 0 {
		return nil
	}

	results := make([]SearchResult, 0, len(vs.Vectors))

	for i := range vs.Vectors {
		similarity := CosineSimilarity(queryEmbedding, vs.Vectors[i].Embedding)
		results = append(results, SearchResult{
			Vector:     &vs.Vectors[i],
			Similarity: similarity,
		})
	}

	// Sort by similarity (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// Return top-K results
	if topK > len(results) {
		topK = len(results)
	}

	return results[:topK]
}

// EstimateTokenCount roughly estimates token count (4 chars ~= 1 token)
func EstimateTokenCount(text string) int {
	return len(text) / 4
}
