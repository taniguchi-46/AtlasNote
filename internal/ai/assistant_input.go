package ai

import (
	"strings"
	"unicode/utf8"
)

const (
	aiMaxQuestionBytes    = 8 * 1024
	aiMaxInstructionBytes = 12 * 1024
	aiMaxHistoryMessages  = textMaxMessages
)

func normalizeAssistantQuestion(value string) (string, error) {
	question := strings.TrimSpace(value)
	if question == "" || !utf8.ValidString(question) || len([]byte(question)) > aiMaxQuestionBytes {
		return "", ErrInputInvalid
	}
	return question, nil
}

func normalizeWritingInstruction(value string) (string, error) {
	instruction := strings.TrimSpace(value)
	if instruction == "" || !utf8.ValidString(instruction) || len([]byte(instruction)) > aiMaxInstructionBytes {
		return "", ErrInputInvalid
	}
	return instruction, nil
}

func normalizeConversationMessages(messages []AIConversationMessage) ([]AIConversationMessage, error) {
	if len(messages) > aiMaxHistoryMessages {
		return nil, ErrInputTooLarge
	}
	result := make([]AIConversationMessage, 0, len(messages))
	messageBytes := 0
	for _, message := range messages {
		role := strings.TrimSpace(message.Role)
		content := strings.TrimSpace(message.Content)
		if (role != "user" && role != "assistant") || content == "" || !utf8.ValidString(content) {
			return nil, ErrInputInvalid
		}
		messageBytes += len([]byte(content))
		if messageBytes > textMessageLimitBytes {
			return nil, ErrInputTooLarge
		}
		result = append(result, AIConversationMessage{Role: role, Content: content})
	}
	return result, nil
}

func conversationMessageBytes(messages []AIConversationMessage) int {
	total := 0
	for _, message := range messages {
		total += len([]byte(message.Content))
	}
	return total
}

func normalizeAssistantKind(kind AssistantKind) (AssistantKind, error) {
	switch kind {
	case AssistantKindQA, AssistantKindBrainstorm:
		return kind, nil
	default:
		return "", ErrInputInvalid
	}
}

func normalizeChatMode(mode ChatMode) (ChatMode, error) {
	switch mode {
	case "", ChatModeAsk:
		return ChatModeAsk, nil
	case ChatModeAgent:
		return ChatModeAgent, nil
	default:
		return "", ErrInputInvalid
	}
}

func normalizeWritingKind(kind WritingKind) (WritingKind, error) {
	switch kind {
	case WritingKindPrompt, WritingKindPromptImprovement, WritingKindREADME, WritingKindDocument, WritingKindBlog, WritingKindRequirements:
		return kind, nil
	default:
		return "", ErrInputInvalid
	}
}
