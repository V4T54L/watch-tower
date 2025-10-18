package domain

import "time"

// ConsumerGroupInfo represents information about a Redis Stream consumer group.
type ConsumerGroupInfo struct {
	Name            string `json:"name"`
	Consumers       int64  `json:"consumers"`
	Pending         int64  `json:"pending"`
	LastDeliveredID string `json:"last_delivered_id"`
}

// ConsumerInfo represents information about a specific consumer in a group.
type ConsumerInfo struct {
	Name    string        `json:"name"`
	Pending int64         `json:"pending"`
	Idle    time.Duration `json:"idle_ms"`
}

// PendingMessageSummary provides a summary of pending messages for a consumer group.
type PendingMessageSummary struct {
	Total          int64            `json:"total"`
	FirstMessageID string           `json:"first_message_id,omitempty"`
	LastMessageID  string           `json:"last_message_id,omitempty"`
	ConsumerTotals map[string]int64 `json:"consumer_totals,omitempty"`
}

// PendingMessageDetail represents a detailed view of a single pending message.
type PendingMessageDetail struct {
	ID         string        `json:"id"`
	Consumer   string        `json:"consumer"`
	IdleTime   time.Duration `json:"idle_time_ms"`
	RetryCount int64         `json:"retry_count"`
}

