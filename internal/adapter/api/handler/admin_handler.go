package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/V4T54L/watch-tower/internal/usecase"
)

// AdminHandler handles HTTP requests for stream administration.
type AdminHandler struct {
	uc     *usecase.AdminStreamUseCase
	logger *slog.Logger
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(uc *usecase.AdminStreamUseCase, logger *slog.Logger) *AdminHandler {
	return &AdminHandler{uc: uc, logger: logger}
}

// HealthCheck is a simple health check endpoint.
func (h *AdminHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// GetGroupInfo handles requests to get consumer group info.
// GET /admin/streams/{streamName}/groups
func (h *AdminHandler) GetGroupInfo(w http.ResponseWriter, r *http.Request) {
	streamName := r.PathValue("streamName")
	if streamName == "" {
		http.Error(w, "streamName is required", http.StatusBadRequest)
		return
	}

	groups, err := h.uc.GetGroupInfo(r.Context(), streamName)
	if err != nil {
		h.logger.Error("failed to get group info", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.respondWithJSON(w, http.StatusOK, groups)
}

// GetConsumerInfo handles requests to get consumer info for a group.
// GET /admin/streams/{streamName}/groups/{groupName}/consumers
func (h *AdminHandler) GetConsumerInfo(w http.ResponseWriter, r *http.Request) {
	streamName := r.PathValue("streamName")
	groupName := r.PathValue("groupName")

	consumers, err := h.uc.GetConsumerInfo(r.Context(), streamName, groupName)
	if err != nil {
		h.logger.Error("failed to get consumer info", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.respondWithJSON(w, http.StatusOK, consumers)
}

// GetPendingSummary handles requests to get a summary of pending messages.
// GET /admin/streams/{streamName}/groups/{groupName}/pending
func (h *AdminHandler) GetPendingSummary(w http.ResponseWriter, r *http.Request) {
	streamName := r.PathValue("streamName")
	groupName := r.PathValue("groupName")

	summary, err := h.uc.GetPendingSummary(r.Context(), streamName, groupName)
	if err != nil {
		h.logger.Error("failed to get pending summary", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.respondWithJSON(w, http.StatusOK, summary)
}

// GetPendingMessages handles requests to list pending messages.
// GET /admin/streams/{streamName}/groups/{groupName}/pending/messages?consumer={consumerName}&start={startID}&count={count}
func (h *AdminHandler) GetPendingMessages(w http.ResponseWriter, r *http.Request) {
	streamName := r.PathValue("streamName")
	groupName := r.PathValue("groupName")
	consumerName := r.URL.Query().Get("consumer")
	startID := r.URL.Query().Get("start")
	countStr := r.URL.Query().Get("count")

	var count int64 = 100 // default
	if countStr != "" {
		var err error
		count, err = strconv.ParseInt(countStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid count parameter", http.StatusBadRequest)
			return
		}
	}

	messages, err := h.uc.GetPendingMessages(r.Context(), streamName, groupName, consumerName, startID, count)
	if err != nil {
		h.logger.Error("failed to get pending messages", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.respondWithJSON(w, http.StatusOK, messages)
}

// ClaimMessages handles requests to claim pending messages.
// POST /admin/streams/{streamName}/groups/{groupName}/claim
func (h *AdminHandler) ClaimMessages(w http.ResponseWriter, r *http.Request) {
	streamName := r.PathValue("streamName")
	groupName := r.PathValue("groupName")

	var payload struct {
		Consumer    string   `json:"consumer"`
		MinIdleTime string   `json:"min_idle_time"`
		MessageIDs  []string `json:"message_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	minIdle, err := time.ParseDuration(payload.MinIdleTime)
	if err != nil {
		http.Error(w, "invalid min_idle_time format", http.StatusBadRequest)
		return
	}

	claimed, err := h.uc.ClaimMessages(r.Context(), streamName, groupName, payload.Consumer, minIdle, payload.MessageIDs)
	if err != nil {
		h.logger.Error("failed to claim messages", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.respondWithJSON(w, http.StatusOK, claimed)
}

// AcknowledgeMessages handles requests to acknowledge messages.
// POST /admin/streams/{streamName}/groups/{groupName}/ack
func (h *AdminHandler) AcknowledgeMessages(w http.ResponseWriter, r *http.Request) {
	streamName := r.PathValue("streamName")
	groupName := r.PathValue("groupName")

	var payload struct {
		MessageIDs []string `json:"message_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(payload.MessageIDs) == 0 {
		http.Error(w, "message_ids cannot be empty", http.StatusBadRequest)
		return
	}

	count, err := h.uc.AcknowledgeMessages(r.Context(), streamName, groupName, payload.MessageIDs...)
	if err != nil {
		h.logger.Error("failed to acknowledge messages", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]int64{"acknowledged": count})
}

// TrimStream handles requests to trim a stream.
// POST /admin/streams/{streamName}/trim
func (h *AdminHandler) TrimStream(w http.ResponseWriter, r *http.Request) {
	streamName := r.PathValue("streamName")

	var payload struct {
		MaxLen int64 `json:"maxlen"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if payload.MaxLen <= 0 {
		http.Error(w, "maxlen must be a positive integer", http.StatusBadRequest)
		return
	}

	trimmedCount, err := h.uc.TrimStream(r.Context(), streamName, payload.MaxLen)
	if err != nil {
		h.logger.Error("failed to trim stream", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]int64{"trimmed": trimmedCount})
}

func (h *AdminHandler) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("failed to marshal JSON response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
