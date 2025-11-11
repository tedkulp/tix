package services

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/tedkulp/tix/internal/logger"
)

// OpenAIResources holds references to OpenAI resources
type OpenAIResources struct {
	Client      *openai.Client
	AssistantID string
	ThreadID    string
	FileID      string
}

// NewOpenAIClient creates and validates the OpenAI client
func NewOpenAIClient() (*openai.Client, error) {
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable is required")
	}
	return openai.NewClient(openaiAPIKey), nil
}

// SetupOpenAIResources creates and initializes OpenAI resources
func SetupOpenAIResources(ctx context.Context, client *openai.Client, diff string) (*OpenAIResources, error) {
	// Create a temporary file for the diff
	tempFile, err := os.CreateTemp("", "mr-diff-*.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	// Write diff to the temporary file
	if err := os.WriteFile(tempFile.Name(), []byte(diff), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write diff to temporary file: %w", err)
	}

	logger.Info("Diff saved to temporary file", map[string]any{
		"file": tempFile.Name(),
		"size": len(diff),
	})

	// Create an assistant
	assistantName := "MR Description Generator"
	instructions := "You are an expert at reviewing code changes and providing concise, informative descriptions."
	assistant, err := client.CreateAssistant(
		ctx,
		openai.AssistantRequest{
			Model: openai.GPT4o,
			Name:  &assistantName,
			Tools: []openai.AssistantTool{
				{
					Type: openai.AssistantToolTypeFileSearch,
				},
			},
			Instructions: &instructions,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI assistant: %w", err)
	}

	logger.Info("Created OpenAI assistant", map[string]any{
		"id": assistant.ID,
	})

	// Upload the file
	fileResp, err := client.CreateFile(ctx, openai.FileRequest{
		FilePath: tempFile.Name(),
		Purpose:  string(openai.PurposeAssistants),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload file to OpenAI: %w", err)
	}

	logger.Info("Uploaded file to OpenAI", map[string]any{
		"file_id": fileResp.ID,
	})

	content := `Generate a concise and informative merge request description based on the following changes:

The full diff is attached as a file.

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
- Areas that might be impacted by these changes
	`

	// Create a thread
	threadReq := openai.ThreadRequest{
		Messages: []openai.ThreadMessage{
			{
				Role:    openai.ThreadMessageRoleUser,
				Content: string(content),
				Attachments: []openai.ThreadAttachment{
					{
						FileID: fileResp.ID,
						Tools: []openai.ThreadAttachmentTool{
							{
								Type: string(openai.AssistantToolTypeFileSearch),
							},
						},
					},
				},
			},
		},
	}
	thread, err := client.CreateThread(ctx, threadReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI thread: %w", err)
	}

	logger.Info("Created OpenAI thread", map[string]any{
		"id": thread.ID,
	})

	return &OpenAIResources{
		Client:      client,
		AssistantID: assistant.ID,
		ThreadID:    thread.ID,
		FileID:      fileResp.ID,
	}, nil
}

// CleanupOpenAIResources deletes OpenAI resources to avoid leaving them dangling
func CleanupOpenAIResources(ctx context.Context, resources *OpenAIResources) {
	if resources == nil || resources.Client == nil {
		return
	}

	if resources.ThreadID != "" {
		_, err := resources.Client.DeleteThread(ctx, resources.ThreadID)
		if err != nil {
			logger.Warn("Failed to delete OpenAI thread", map[string]any{
				"error": err.Error(),
			})
		}
		logger.Debug("Deleted OpenAI thread", map[string]any{
			"id": resources.ThreadID,
		})
	}

	if resources.AssistantID != "" {
		_, err := resources.Client.DeleteAssistant(ctx, resources.AssistantID)
		if err != nil {
			logger.Warn("Failed to delete OpenAI assistant", map[string]any{
				"error": err.Error(),
			})
		}
		logger.Debug("Deleted OpenAI assistant", map[string]any{
			"id": resources.AssistantID,
		})
	}

	if resources.FileID != "" {
		err := resources.Client.DeleteFile(ctx, resources.FileID)
		if err != nil {
			logger.Warn("Failed to delete OpenAI file", map[string]any{
				"error": err.Error(),
			})
		}
		logger.Debug("Deleted OpenAI file", map[string]any{
			"id": resources.FileID,
		})
	}
}

// PollRun polls the OpenAI API until a run is completed
func PollRun(ctx context.Context, client *openai.Client, threadID, runID string) (*openai.Run, error) {
	for {
		run, err := client.RetrieveRun(ctx, threadID, runID)
		if err != nil {
			return nil, err
		}

		if run.Status == openai.RunStatusCompleted {
			return &run, nil
		}

		if run.Status == openai.RunStatusFailed || run.Status == openai.RunStatusCancelled || run.Status == openai.RunStatusExpired {
			return nil, fmt.Errorf("run failed with status: %s", run.Status)
		}

		// Wait before polling again
		logger.Debug("Waiting for OpenAI run to complete", map[string]any{
			"status": run.Status,
		})
		time.Sleep(1 * time.Second)
	}
}

// GenerateMRDescription generates a description for a merge request using OpenAI
func GenerateMRDescription(ctx context.Context, resources *OpenAIResources) (string, error) {
	// Create a run
	run, err := resources.Client.CreateRun(ctx, resources.ThreadID, openai.RunRequest{
		AssistantID: resources.AssistantID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create OpenAI run: %w", err)
	}

	logger.Info("Created OpenAI run", map[string]any{
		"id": run.ID,
	})

	// Poll until the run is completed
	runResult, err := PollRun(ctx, resources.Client, resources.ThreadID, run.ID)
	if err != nil {
		return "", fmt.Errorf("failed to wait for OpenAI run completion: %w", err)
	}
	run = *runResult

	// Get the messages
	limit := 10
	order := "desc"
	messages, err := resources.Client.ListMessage(ctx, resources.ThreadID, &limit, &order, nil, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to list OpenAI messages: %w", err)
	}

	// The last message should be the assistant's response
	var description string
	if len(messages.Messages) > 1 {
		// Check message content type and extract text
		for _, contentItem := range messages.Messages[0].Content {
			if contentItem.Type == "text" {
				description = contentItem.Text.Value
				break
			}
		}
	}

	if description == "" {
		return "", fmt.Errorf("failed to get a description from OpenAI")
	}

	return description, nil
}

// GenerateIssueDescription generates a description for an issue using OpenAI
func GenerateIssueDescription(ctx context.Context, resources *OpenAIResources, currentTitle string) (string, string, error) {
	content := fmt.Sprintf(`
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
issue is about the intended behavior or outcome, and not the inpelementation or code specifics.

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
	`, currentTitle, currentTitle)

	// Create a message asking for an issue description
	_, err := resources.Client.CreateMessage(ctx, resources.ThreadID, openai.MessageRequest{
		Role:    string(openai.ThreadMessageRoleUser),
		Content: content,
		Attachments: []openai.ThreadAttachment{
			{
				FileID: resources.FileID,
				Tools: []openai.ThreadAttachmentTool{
					{
						Type: string(openai.AssistantToolTypeFileSearch),
					},
				},
			},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to create OpenAI message for issue description: %w", err)
	}

	// Create a run for the issue description
	run, err := resources.Client.CreateRun(ctx, resources.ThreadID, openai.RunRequest{
		AssistantID: resources.AssistantID,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to create OpenAI run for issue description: %w", err)
	}

	// Poll until the run is completed
	runResult, err := PollRun(ctx, resources.Client, resources.ThreadID, run.ID)
	if err != nil {
		return "", "", fmt.Errorf("failed to wait for OpenAI run completion: %w", err)
	}
	run = *runResult

	// Get the messages
	limit := 10
	order := "desc"
	messages, err := resources.Client.ListMessage(ctx, resources.ThreadID, &limit, &order, nil, nil, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to list OpenAI messages: %w", err)
	}

	// The first message should now be the assistant's response for the issue description
	var description string
	if len(messages.Messages) > 0 {
		// Check message content type and extract text
		for _, contentItem := range messages.Messages[0].Content {
			if contentItem.Type == "text" {
				description = contentItem.Text.Value
				break
			}
		}
	}

	if description == "" {
		return "", "", fmt.Errorf("failed to get an issue description from OpenAI")
	}

	// Check if the first line contains a title in markdown format
	title := ""
	if strings.HasPrefix(description, "## ") {
		lines := strings.Split(description, "\n")
		title = strings.TrimSpace(strings.TrimPrefix(lines[0], "## "))
		description = strings.TrimSpace(strings.Join(lines[1:], "\n"))
	}

	return title, description, nil
}
