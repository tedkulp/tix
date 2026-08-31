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
	return fmt.Sprintf(`Generate a brief merge request description based on the following changes. Be concise - prefer 1-2 bullet points per section rather than exhaustive lists.

%s

Do not include any inline citations, references, or source markers such as 【x:x†source】 in the output.

Please format the description EXACTLY in the following structure:

### Summary

1-3 sentences explaining what changed and why.

### For Developers

Key technical changes (1-5 bullet points):
- Only mention significant changes, new components, breaking changes or SERIOUS performance considerations
- Skip obvious implementation details

### For Quality

What to test (1-5 bullet points with markdown checkboxes):
- [ ] Focus on user-facing changes and critical paths
- [ ] Skip testing details that are self-evident`, diffContent)
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

// descriptionModel is the chat model used for every generated description.
const descriptionModel = "gpt-5.6-luna"

// shouldUseRAG decides between the RAG and direct paths, honouring an explicit
// override and otherwise falling back to the size of the diff.
func shouldUseRAG(forceRAG *bool, diff string) bool {
	if forceRAG != nil {
		if *forceRAG {
			logger.Info("Using RAG approach (forced by --use-rag flag)")
		} else {
			logger.Info("Using direct approach (forced by --use-rag=false flag)")
		}
		return *forceRAG
	}

	const smallDiffThreshold = 50000 // estimated tokens, so ~200k characters
	if EstimateTokenCount(diff) >= smallDiffThreshold {
		logger.Info("Diff is large, using RAG approach with embeddings")
		return true
	}
	logger.Info("Diff is small enough, using direct approach without RAG")
	return false
}

// generateDescription runs a diff through the model, either directly or via RAG.
// prompt wraps the content the model sees; query is the RAG retrieval query and
// is unused on the direct path.
func generateDescription(ctx context.Context, client *openai.Client, diff string, forceRAG *bool, prompt func(string) string, query string) (string, error) {
	content := diff
	if shouldUseRAG(forceRAG, diff) {
		var err error
		if content, err = retrieveContext(ctx, client, diff, query); err != nil {
			return "", err
		}
	}

	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: descriptionModel,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt(content),
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

// retrieveContext chunks the diff, embeds it, and returns the chunks most
// relevant to query as a single prompt-ready block.
func retrieveContext(ctx context.Context, client *openai.Client, diff string, query string) (string, error) {
	chunks := ChunkDiff(diff)
	if len(chunks) == 0 {
		return "", fmt.Errorf("no chunks generated from diff")
	}

	logger.Info("Processing diff with RAG", map[string]any{
		"chunks": len(chunks),
	})

	embeddings, err := GenerateEmbeddings(ctx, client, chunks)
	if err != nil {
		return "", fmt.Errorf("failed to generate embeddings: %w", err)
	}

	queryResp, err := client.CreateEmbeddings(ctx, openai.EmbeddingRequestStrings{
		Input: []string{query},
		Model: openai.SmallEmbedding3,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate query embedding: %w", err)
	}

	if len(queryResp.Data) == 0 {
		return "", fmt.Errorf("no query embedding returned")
	}

	topK := 15
	if topK > len(chunks) {
		topK = len(chunks)
	}
	results := NewVectorStore(embeddings).Search(queryResp.Data[0].Embedding, topK)

	var contextBuilder strings.Builder
	contextBuilder.WriteString("Here are the most relevant code changes:\n\n")

	for i, result := range results {
		fmt.Fprintf(&contextBuilder, "--- Change %d (similarity: %.3f) ---\n", i+1, result.Similarity)
		if result.Vector.Chunk.FilePath != "" {
			fmt.Fprintf(&contextBuilder, "File: %s\n", result.Vector.Chunk.FilePath)
		}
		contextBuilder.WriteString(result.Vector.Chunk.Content)
		contextBuilder.WriteString("\n\n")
	}

	return contextBuilder.String(), nil
}

// extractTitle splits a leading "## " heading off a generated description.
func extractTitle(description string) (string, string) {
	if !strings.HasPrefix(description, "## ") {
		return "", description
	}
	lines := strings.Split(description, "\n")
	return strings.TrimSpace(strings.TrimPrefix(lines[0], "## ")),
		strings.TrimSpace(strings.Join(lines[1:], "\n"))
}

// GenerateMRDescriptionWithOptions generates a description with optional RAG override
func GenerateMRDescriptionWithOptions(ctx context.Context, client *openai.Client, diff string, forceRAG *bool) (string, error) {
	return generateDescription(ctx, client, diff, forceRAG, buildMRPrompt, `Generate a concise merge request description that includes:
- A summary of the changes
- Technical details for developers
- Testing information for QA`)
}

// GenerateIssueDescriptionWithOptions generates an issue description with optional RAG override
func GenerateIssueDescriptionWithOptions(ctx context.Context, client *openai.Client, diff string, currentTitle string, forceRAG *bool) (string, string, error) {
	prompt := func(content string) string { return buildIssuePrompt(content, currentTitle) }
	query := fmt.Sprintf(`Generate an issue description for: %s
Focus on the business logic changes, the intended behavior, and the motivation for the change.`, currentTitle)

	description, err := generateDescription(ctx, client, diff, forceRAG, prompt, query)
	if err != nil {
		return "", "", err
	}

	title, description := extractTitle(description)
	return title, description, nil
}
