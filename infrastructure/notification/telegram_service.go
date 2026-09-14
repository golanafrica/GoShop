package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// TelegramService implémente l'envoi de notifications via l'API Telegram
type TelegramService struct {
	botToken    string
	adminChatID string // 🆕 ID du chat de l'administrateur (pour les tests et alertes)
	httpClient  *http.Client
	logger      *zerolog.Logger
}

// NewTelegramService crée une nouvelle instance du service Telegram
func NewTelegramService(botToken, adminChatID string, logger *zerolog.Logger) *TelegramService {
	return &TelegramService{
		botToken:    botToken,
		adminChatID: adminChatID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// TelegramMessage représente le payload envoyé à l'API Telegram
type TelegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"` // "Markdown" ou "HTML"
}

// SendMessage envoie un message à un Chat ID spécifique
func (s *TelegramService) SendMessage(ctx context.Context, chatID, message string) error {
	if s.botToken == "" {
		s.logger.Warn().Msg("Telegram bot token is not configured, skipping notification")
		return nil
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)

	payload := TelegramMessage{
		ChatID:    chatID,
		Text:      message,
		ParseMode: "HTML", // Permet d'utiliser <b>gras</b>, <i>italique</i>, etc.
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal telegram message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	s.logger.Info().Str("chat_id", chatID).Msg("Telegram notification sent successfully")
	return nil
}

// SendAdminNotification envoie une notification à l'administrateur configuré
func (s *TelegramService) SendAdminNotification(ctx context.Context, title, message string, data map[string]interface{}) error {
	if s.adminChatID == "" {
		return fmt.Errorf("admin chat ID not configured")
	}

	telegramMsg := fmt.Sprintf("<b>🚨 ADMIN: %s</b>\n\n%s", title, message)
	if len(data) > 0 {
		telegramMsg += "\n📊 <b>Détails :</b>\n"
		for key, value := range data {
			telegramMsg += fmt.Sprintf("• %s : %v\n", key, value)
		}
	}

	return s.SendMessage(ctx, s.adminChatID, telegramMsg)
}
