package handler

import (
	"fmt"
	"log"

	"github.com/asb1302/innopolis_go_assesment_1/internal/config"
	"github.com/asb1302/innopolis_go_assesment_1/internal/repository"
	"github.com/asb1302/innopolis_go_assesment_1/internal/types"
)

type MessageHandler struct {
	userRepo *repository.UserRepository
	app      types.AppInterface
	cfg      *config.Config
}

func NewMessageHandler(userRepo *repository.UserRepository, app types.AppInterface, cfg *config.Config) *MessageHandler {
	return &MessageHandler{
		userRepo: userRepo,
		app:      app,
		cfg:      cfg,
	}
}

func (h *MessageHandler) HandleMessage(msg types.Message) error {
	log.Printf("обработка сообщения для файла %s", msg.FileID)

	if !h.userRepo.IsValidToken(msg.Token) {
		log.Printf("неверный токен для сообщения: %+v", msg)
		return fmt.Errorf("invalid token")
	}

	h.app.SendMsg(msg)
	return nil
}
