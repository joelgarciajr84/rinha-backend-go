package handler

import (
	"log/slog"

	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v2"

	"rinha/adapter"
	"rinha/model"
)

type PaymentHandler struct {
	adapter *adapter.PaymentProcessorAdapter
}

func NewPaymentHandler(a *adapter.PaymentProcessorAdapter) *PaymentHandler {
	return &PaymentHandler{adapter: a}
}

func (h *PaymentHandler) Process(c *fiber.Ctx) error {
	var req model.PaymentRequest
	if err := sonic.Unmarshal(c.Body(), &req); err != nil || req.CorrelationId == "" || req.Amount <= 0 {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	// slog.Info("Received payment request", "correlationId", req.CorrelationId, "amount", req.Amount)
	go h.adapter.Process(model.PaymentRequestProcessor{PaymentRequest: req})
	return c.SendStatus(fiber.StatusAccepted)
}

func (h *PaymentHandler) Summary(c *fiber.Ctx) error {
	slog.Info("Fetching payment summary", "method", c.Method(), "path", c.Path(), "from", c.Query("from"), "to", c.Query("to"))
	summary, err := h.adapter.Summary(c.Query("from"), c.Query("to"), "")
	if err != nil {
		slog.Error("Error generating summary", "err", err)
		return c.Status(fiber.StatusInternalServerError).JSON(model.SummaryResponse{
			DefaultSummary:  model.SummaryTotalRequestsResponse{},
			FallbackSummary: model.SummaryTotalRequestsResponse{},
		})
	}

	slog.Info("Summary result", "default_requests", summary.DefaultSummary.TotalRequests, "default_amount", summary.DefaultSummary.TotalAmount, "fallback_requests", summary.FallbackSummary.TotalRequests, "fallback_amount", summary.FallbackSummary.TotalAmount)
	return c.JSON(summary)
}

func (h *PaymentHandler) Purge(c *fiber.Ctx) error {
	slog.Info("Purging payments", "method", c.Method(), "path", c.Path())
	if err := h.adapter.Purge(c.Get("X-Rinha-Token", "123")); err != nil {
		slog.Error("Error during purge", "err", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.SendStatus(fiber.StatusOK)
}
