package services

import (
	"context"
	"fmt"
	"os"
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
	if err := os.WriteFile(tempFile.Name(), []byte(diff), 0644); err != nil {
		return nil, fmt.Errorf("failed to write diff to temporary file: %w", err)
	}

	logger.Info("Diff saved to temporary file", map[string]interface{}{
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

	logger.Info("Created OpenAI assistant", map[string]interface{}{
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

	logger.Info("Uploaded file to OpenAI", map[string]interface{}{
		"file_id": fileResp.ID,
	})

	content := "Please describe the changes in the attached git diff in plain English."

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

	logger.Info("Created OpenAI thread", map[string]interface{}{
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

	if resources.AssistantID != "" {
		_, err := resources.Client.DeleteAssistant(ctx, resources.AssistantID)
		if err != nil {
			logger.Warn("Failed to delete OpenAI assistant", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	if resources.FileID != "" {
		err := resources.Client.DeleteFile(ctx, resources.FileID)
		if err != nil {
			logger.Warn("Failed to delete OpenAI file", map[string]interface{}{
				"error": err.Error(),
			})
		}
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
		logger.Debug("Waiting for OpenAI run to complete", map[string]interface{}{
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

	logger.Info("Created OpenAI run", map[string]interface{}{
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
func GenerateIssueDescription(ctx context.Context, resources *OpenAIResources) (string, error) {
	// Create a message asking for an issue description
	_, err := resources.Client.CreateMessage(ctx, resources.ThreadID, openai.MessageRequest{
		Role:    string(openai.ThreadMessageRoleUser),
		Content: "Please write a description for the related issue as if the code hadn't been written yet. This should describe the problem, requirements, and goals that this code is solving. Keep it concise but comprehensive.",
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
		return "", fmt.Errorf("failed to create OpenAI message for issue description: %w", err)
	}

	// Create a run for the issue description
	run, err := resources.Client.CreateRun(ctx, resources.ThreadID, openai.RunRequest{
		AssistantID: resources.AssistantID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create OpenAI run for issue description: %w", err)
	}

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
		return "", fmt.Errorf("failed to get an issue description from OpenAI")
	}

	return description, nil
}
