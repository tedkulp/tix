package services

import (
	"context"
	"fmt"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	"github.com/tedkulp/tix/internal/logger"
)

// NewOpenAIClient creates and validates the OpenAI client
func NewOpenAIClient() (*openai.Client, error) {
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable is required")
	}
	return openai.NewClient(openaiAPIKey), nil
}

// buildMRPrompt creates the prompt for MR description generation
func buildMRPrompt(diffContent string) string {
	return fmt.Sprintf(`Generate a concise and informative merge request description based on the following changes:

%s

Do not include any inline citations, references, or source markers such as 【x:x†source】 in the output.

Please format the description EXACTLY in the following structure:

### Summary

A clear and concise summary of the changes (1-3 sentences). Focus on what was done and why.

### For Developers

Technical details about implementation, architecture changes, and code modifications. Include:
- Major code changes and their purpose
- New components or modules added
- Any performance considerations
- Breaking changes or deprecations

### For Quality

Information relevant for testers and QA:
- What should be tested
- Potential edge cases to consider
- Any specific testing procedures required
- Areas that might be impacted by these changes`, diffContent)
}

// buildIssuePrompt creates the prompt for issue description generation
func buildIssuePrompt(diffContent string, currentTitle string) string {
	return fmt.Sprintf(`
Given the following diff of code changes, write a GitLab issue description that outlines
what needs to change and why — as if it were written before the code was implemented. The
description should explain the motivation for the change, the intended behavior or outcome,
and any constraints or considerations, but should avoid describing the actual implementation
or code specifics. Assume the reader is a teammate reviewing this before any work has been
started.

The current issue title is: "%s"

You can either keep this title or suggest a better one. If you suggest a new title, make sure
it's clear, concise, and accurately reflects the changes being made.

Keep in mind, however, that this current title was the original intention of the change and
the new title and description should reflect that. Meaning that, if the original intention
was a business logic change and it required a huge refactoring that uses a new technology
or paradigm, still keep the original intention as the focus of the generated text. Again, the
issue is about the intended behavior or outcome, and not the implementation or code specifics.

%s

Please format the description EXACTLY in the following structure:

## <Put the title here. Keep the current title "%s" or suggest a better one. It shouldn't be over 200 characters.>

### Summary
	
A clear and concise summary of the changes (1-3 sentences). Focus on what needs to change
and why.
	
### Rationale
	
The rationale for the change. Again, it should be 1-3 sentences, clear and concise.

### Acceptance Criteria

- [ ] High-level acceptance criteria or goals
- [ ] They shouldn't mention specific file names, functions, or code
- [ ] They should be in markdown checkboxes
- [ ] They should assume the reader is a teammate reviewing this before any work has been started.
	`, currentTitle, diffContent, currentTitle)
}

// GenerateMRDescription generates a description for a merge request using OpenAI with RAG
func GenerateMRDescription(ctx context.Context, client *openai.Client, diff string) (string, error) {
	return GenerateMRDescriptionWithOptions(ctx, client, diff, nil)
}

// GenerateMRDescriptionWithOptions generates a description with optional RAG override
func GenerateMRDescriptionWithOptions(ctx context.Context, client *openai.Client, diff string, forceRAG *bool) (string, error) {
	// Determine whether to use RAG
	useRAG := false
	if forceRAG != nil {
		// User explicitly requested RAG on or off
		useRAG = *forceRAG
		if useRAG {
			logger.Info("Using RAG approach (forced by --use-rag flag)")
		} else {
			logger.Info("Using direct approach (forced by --use-rag=false flag)")
		}
	} else {
		// Auto-detect based on diff size
		const smallDiffThreshold = 50000 // ~12.5k tokens
		useRAG = EstimateTokenCount(diff) >= smallDiffThreshold
		if useRAG {
			logger.Info("Diff is large, using RAG approach with embeddings")
		} else {
			logger.Info("Diff is small enough, using direct approach without RAG")
		}
	}

	if useRAG {
		return generateMRDescriptionWithRAG(ctx, client, diff)
	}
	return generateMRDescriptionDirect(ctx, client, diff)
}

// generateMRDescriptionDirect generates a description directly without RAG for small diffs
func generateMRDescriptionDirect(ctx context.Context, client *openai.Client, diff string) (string, error) {
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT5Mini,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: buildMRPrompt(diff),
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate description: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}

// generateMRDescriptionWithRAG generates a description using RAG for large diffs
func generateMRDescriptionWithRAG(ctx context.Context, client *openai.Client, diff string) (string, error) {
	// Step 1: Chunk the diff
	chunks := ChunkDiff(diff)
	if len(chunks) == 0 {
		return "", fmt.Errorf("no chunks generated from diff")
	}

	logger.Info("Processing diff with RAG", map[string]any{
		"chunks": len(chunks),
	})

	// Step 2: Generate embeddings for all chunks
	embeddings, err := GenerateEmbeddings(ctx, client, chunks)
	if err != nil {
		return "", fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Step 3: Create vector store
	vectorStore := NewVectorStore(embeddings)

	// Step 4: Create query for what we're looking for
	queryText := `Generate a concise merge request description that includes:
- A summary of the changes
- Technical details for developers
- Testing information for QA`

	// Generate embedding for the query
	queryResp, err := client.CreateEmbeddings(ctx, openai.EmbeddingRequestStrings{
		Input: []string{queryText},
		Model: openai.SmallEmbedding3,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate query embedding: %w", err)
	}

	if len(queryResp.Data) == 0 {
		return "", fmt.Errorf("no query embedding returned")
	}

	// Step 5: Search for most relevant chunks
	topK := 15
	if topK > len(chunks) {
		topK = len(chunks)
	}
	results := vectorStore.Search(queryResp.Data[0].Embedding, topK)

	// Step 6: Build context from retrieved chunks
	var contextBuilder strings.Builder
	contextBuilder.WriteString("Here are the most relevant code changes:\n\n")

	for i, result := range results {
		contextBuilder.WriteString(fmt.Sprintf("--- Change %d (similarity: %.3f) ---\n", i+1, result.Similarity))
		if result.Vector.Chunk.FilePath != "" {
			contextBuilder.WriteString(fmt.Sprintf("File: %s\n", result.Vector.Chunk.FilePath))
		}
		contextBuilder.WriteString(result.Vector.Chunk.Content)
		contextBuilder.WriteString("\n\n")
	}

	// Step 7: Generate description using retrieved context
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: buildMRPrompt(contextBuilder.String()),
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate description: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}

// GenerateIssueDescription generates a description for an issue using OpenAI with RAG
func GenerateIssueDescription(ctx context.Context, client *openai.Client, diff string, currentTitle string) (string, string, error) {
	return GenerateIssueDescriptionWithOptions(ctx, client, diff, currentTitle, nil)
}

// GenerateIssueDescriptionWithOptions generates an issue description with optional RAG override
func GenerateIssueDescriptionWithOptions(ctx context.Context, client *openai.Client, diff string, currentTitle string, forceRAG *bool) (string, string, error) {
	// Determine whether to use RAG
	useRAG := false
	if forceRAG != nil {
		// User explicitly requested RAG on or off
		useRAG = *forceRAG
		if useRAG {
			logger.Info("Using RAG approach (forced by --use-rag flag)")
		} else {
			logger.Info("Using direct approach (forced by --use-rag=false flag)")
		}
	} else {
		// Auto-detect based on diff size
		const smallDiffThreshold = 50000 // ~12.5k tokens
		useRAG = EstimateTokenCount(diff) >= smallDiffThreshold
		if useRAG {
			logger.Info("Diff is large, using RAG approach with embeddings")
		} else {
			logger.Info("Diff is small enough, using direct approach without RAG")
		}
	}

	if useRAG {
		return generateIssueDescriptionWithRAG(ctx, client, diff, currentTitle)
	}
	return generateIssueDescriptionDirect(ctx, client, diff, currentTitle)
}

// generateIssueDescriptionDirect generates an issue description directly without RAG for small diffs
func generateIssueDescriptionDirect(ctx context.Context, client *openai.Client, diff string, currentTitle string) (string, string, error) {
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: buildIssuePrompt(diff, currentTitle),
			},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to generate issue description: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", "", fmt.Errorf("no response from OpenAI")
	}

	description := resp.Choices[0].Message.Content

	// Extract title if present
	title := ""
	if strings.HasPrefix(description, "## ") {
		lines := strings.Split(description, "\n")
		title = strings.TrimSpace(strings.TrimPrefix(lines[0], "## "))
		description = strings.TrimSpace(strings.Join(lines[1:], "\n"))
	}

	return title, description, nil
}

// generateIssueDescriptionWithRAG generates an issue description using RAG for large diffs
func generateIssueDescriptionWithRAG(ctx context.Context, client *openai.Client, diff string, currentTitle string) (string, string, error) {
	// Step 1: Chunk the diff
	chunks := ChunkDiff(diff)
	if len(chunks) == 0 {
		return "", "", fmt.Errorf("no chunks generated from diff")
	}

	logger.Info("Processing diff with RAG for issue description", map[string]any{
		"chunks": len(chunks),
	})

	// Step 2: Generate embeddings for all chunks
	embeddings, err := GenerateEmbeddings(ctx, client, chunks)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Step 3: Create vector store
	vectorStore := NewVectorStore(embeddings)

	// Step 4: Create query for what we're looking for
	queryText := fmt.Sprintf(`Generate an issue description for: %s
Focus on the business logic changes, the intended behavior, and the motivation for the change.`, currentTitle)

	// Generate embedding for the query
	queryResp, err := client.CreateEmbeddings(ctx, openai.EmbeddingRequestStrings{
		Input: []string{queryText},
		Model: openai.SmallEmbedding3,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to generate query embedding: %w", err)
	}

	if len(queryResp.Data) == 0 {
		return "", "", fmt.Errorf("no query embedding returned")
	}

	// Step 5: Search for most relevant chunks
	topK := 15
	if topK > len(chunks) {
		topK = len(chunks)
	}
	results := vectorStore.Search(queryResp.Data[0].Embedding, topK)

	// Step 6: Build context from retrieved chunks
	var contextBuilder strings.Builder
	contextBuilder.WriteString("Here are the most relevant code changes:\n\n")

	for i, result := range results {
		contextBuilder.WriteString(fmt.Sprintf("--- Change %d (similarity: %.3f) ---\n", i+1, result.Similarity))
		if result.Vector.Chunk.FilePath != "" {
			contextBuilder.WriteString(fmt.Sprintf("File: %s\n", result.Vector.Chunk.FilePath))
		}
		contextBuilder.WriteString(result.Vector.Chunk.Content)
		contextBuilder.WriteString("\n\n")
	}

	// Step 7: Generate issue description using retrieved context
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: buildIssuePrompt(contextBuilder.String(), currentTitle),
			},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to generate issue description: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", "", fmt.Errorf("no response from OpenAI")
	}

	description := resp.Choices[0].Message.Content

	// Extract title if present
	title := ""
	if strings.HasPrefix(description, "## ") {
		lines := strings.Split(description, "\n")
		title = strings.TrimSpace(strings.TrimPrefix(lines[0], "## "))
		description = strings.TrimSpace(strings.Join(lines[1:], "\n"))
	}

	return title, description, nil
}
