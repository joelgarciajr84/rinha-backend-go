package handler

import (
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

	go h.adapter.Process(model.PaymentRequestProcessor{PaymentRequest: req})
	return c.SendStatus(fiber.StatusAccepted)
}

func (h *PaymentHandler) Summary(c *fiber.Ctx) error {
	summary, err := h.adapter.Summary(c.Query("from"), c.Query("to"), c.Get("X-Rinha-Token", "123"))
	if err != nil {
		return c.Status(fiber.StatusOK).JSON(model.SummaryResponse{
			DefaultSummary:  model.SummaryTotalRequestsResponse{},
			FallbackSummary: model.SummaryTotalRequestsResponse{},
		})
	}

	return c.JSON(summary)
}

func (h *PaymentHandler) Purge(c *fiber.Ctx) error {
	if err := h.adapter.Purge(c.Get("X-Rinha-Token", "123")); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.SendStatus(fiber.StatusOK)
}
