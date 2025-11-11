package services

import (
	"strings"
	"testing"
)

func TestChunkDiff(t *testing.T) {
	// Test with a simple diff
	diff := `diff --git a/file1.go b/file1.go
index 123..456 100644
--- a/file1.go
+++ b/file1.go
@@ -1,5 +1,5 @@
 package main
 
-func old() {
+func new() {
     return "hello"
 }
diff --git a/file2.go b/file2.go
index 789..012 100644
--- a/file2.go
+++ b/file2.go
@@ -1,3 +1,3 @@
 package test
-// old comment
+// new comment
`

	chunks := ChunkDiff(diff)

	if len(chunks) == 0 {
		t.Fatal("Expected at least one chunk, got none")
	}

	// Verify chunks contain content
	for i, chunk := range chunks {
		if strings.TrimSpace(chunk.Content) == "" {
			t.Errorf("Chunk %d has empty content", i)
		}
		if chunk.Index != i {
			t.Errorf("Chunk %d has incorrect index: %d", i, chunk.Index)
		}
	}

	// Verify the chunks combined contain the original diff
	var combined strings.Builder
	for _, chunk := range chunks {
		combined.WriteString(chunk.Content)
	}

	combinedStr := combined.String()
	if !strings.Contains(combinedStr, "func new()") {
		t.Error("Combined chunks don't contain expected content")
	}
}

func TestChunkDiffLarge(t *testing.T) {
	// Create a large diff with many lines
	var builder strings.Builder
	builder.WriteString("diff --git a/large.go b/large.go\n")
	builder.WriteString("--- a/large.go\n")
	builder.WriteString("+++ b/large.go\n")

	// Add 2000 lines to exceed chunk size
	for i := 0; i < 2000; i++ {
		builder.WriteString("+ line\n")
	}

	chunks := ChunkDiff(builder.String())

	// Should be split into multiple chunks
	if len(chunks) < 2 {
		t.Errorf("Expected at least 2 chunks for large diff, got %d", len(chunks))
	}

	// Verify each chunk is not too large
	for i, chunk := range chunks {
		lineCount := strings.Count(chunk.Content, "\n")
		if lineCount > 1000 {
			t.Errorf("Chunk %d has %d lines, expected <= 1000", i, lineCount)
		}
	}
}

func TestCosineSimilarity(t *testing.T) {
	// Test identical vectors
	v1 := []float32{1.0, 2.0, 3.0}
	v2 := []float32{1.0, 2.0, 3.0}

	sim := CosineSimilarity(v1, v2)
	if sim < 0.99 || sim > 1.01 {
		t.Errorf("Expected similarity ~1.0 for identical vectors, got %f", sim)
	}

	// Test orthogonal vectors
	v3 := []float32{1.0, 0.0, 0.0}
	v4 := []float32{0.0, 1.0, 0.0}

	sim2 := CosineSimilarity(v3, v4)
	if sim2 < -0.01 || sim2 > 0.01 {
		t.Errorf("Expected similarity ~0.0 for orthogonal vectors, got %f", sim2)
	}

	// Test opposite vectors
	v5 := []float32{1.0, 0.0, 0.0}
	v6 := []float32{-1.0, 0.0, 0.0}

	sim3 := CosineSimilarity(v5, v6)
	if sim3 > -0.99 || sim3 < -1.01 {
		t.Errorf("Expected similarity ~-1.0 for opposite vectors, got %f", sim3)
	}
}

func TestVectorStoreSearch(t *testing.T) {
	// Create mock embeddings
	chunks := []Chunk{
		{Content: "chunk 1", Index: 0, Lines: 10},
		{Content: "chunk 2", Index: 1, Lines: 20},
		{Content: "chunk 3", Index: 2, Lines: 15},
	}

	vectors := []EmbeddingVector{
		{Chunk: &chunks[0], Embedding: []float32{1.0, 0.0, 0.0}},
		{Chunk: &chunks[1], Embedding: []float32{0.0, 1.0, 0.0}},
		{Chunk: &chunks[2], Embedding: []float32{0.5, 0.5, 0.0}},
	}

	store := NewVectorStore(vectors)

	// Search for vector similar to first chunk
	query := []float32{0.9, 0.1, 0.0}
	results := store.Search(query, 2)

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// First result should be most similar (chunk 0)
	if results[0].Vector.Chunk.Index != 0 {
		t.Errorf("Expected first result to be chunk 0, got chunk %d", results[0].Vector.Chunk.Index)
	}

	// Results should be sorted by similarity
	if results[0].Similarity < results[1].Similarity {
		t.Error("Results are not sorted by similarity in descending order")
	}
}

func TestEstimateTokenCount(t *testing.T) {
	tests := []struct {
		text     string
		expected int
	}{
		{"hello", 1},                          // 5 chars / 4 = 1
		{"hello world", 2},                    // 11 chars / 4 = 2
		{"this is a longer piece of text", 7}, // 31 chars / 4 = 7
	}

	for _, tt := range tests {
		result := EstimateTokenCount(tt.text)
		if result != tt.expected {
			t.Errorf("EstimateTokenCount(%q) = %d, want %d", tt.text, result, tt.expected)
		}
	}
}
