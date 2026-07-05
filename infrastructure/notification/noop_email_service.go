package notification

import (
	"fmt"
	"time"

	"Goshop/domain/service"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.3.2 : NOOP EMAIL SERVICE
// ============================================================
//
// 🎯 Objectif :
//   Implémenter l'interface EmailService en mode "no-op" (ne fait rien).
//   Utilisé comme fallback quand le service SMTP n'est pas configuré.
//   Log tous les appels pour debug.
//
// ============================================================

// NoopEmailService implémente EmailService en mode no-op
type NoopEmailService struct {
	logger zerolog.Logger
}

// NewNoopEmailService crée une nouvelle instance du service no-op
func NewNoopEmailService(logger zerolog.Logger) *NoopEmailService {
	logger.Info().
		Str("component", "noop_email_service").
		Msg("✅ Noop email service initialized (emails will be logged, not sent)")

	return &NoopEmailService{
		logger: logger.With().Str("component", "noop_email_service").Logger(),
	}
}

// ============================================================
// MÉTHODES GÉNÉRIQUES
// ============================================================

// SendEmail log l'email sans l'envoyer
func (s *NoopEmailService) SendEmail(message *service.EmailMessage) (*service.EmailResult, error) {
	if message == nil {
		return nil, service.ErrEmailInvalidBody
	}

	s.logger.Info().
		Str("to", message.To).
		Str("to_name", message.ToName).
		Str("subject", message.Subject).
		Int("html_length", len(message.HTMLBody)).
		Int("text_length", len(message.TextBody)).
		Int("attachments", len(message.Attachments)).
		Msg("📧 [NOOP] Email non envoyé (service no-op)")

	s.logger.Debug().
		Str("html_preview", truncate(message.HTMLBody, 500)).
		Msg("📄 Contenu HTML de l'email")

	return &service.EmailResult{
		MessageID: fmt.Sprintf("noop-%s@goshop.local", uuid.New().String()[:8]),
		SentAt:    time.Now(),
		Provider:  "noop",
	}, nil
}

// SendEmailAsync log l'email sans l'envoyer (asynchrone)
func (s *NoopEmailService) SendEmailAsync(message *service.EmailMessage) {
	go func() {
		result, err := s.SendEmail(message)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("to", message.To).
				Str("subject", message.Subject).
				Msg("❌ Erreur envoi email asynchrone (no-op)")
			return
		}

		s.logger.Debug().
			Str("message_id", result.MessageID).
			Str("to", message.To).
			Msg("✅ Email asynchrone traité (no-op)")
	}()
}

// ============================================================
// MÉTHODES SPÉCIFIQUES AUX TEMPLATES
// ============================================================

// SendPlatformInvitation log l'invitation sans l'envoyer
func (s *NoopEmailService) SendPlatformInvitation(data *service.PlatformInvitationData) (*service.EmailResult, error) {
	if data == nil {
		return nil, service.ErrEmailInvalidRecipient
	}

	s.logger.Info().
		Str("to", data.RecipientEmail).
		Str("role", data.Role).
		Str("role_display", data.RoleDisplayName).
		Str("inviter", data.InviterName).
		Str("invitation_id", data.InvitationID).
		Msg("📧 [NOOP] Invitation plateforme non envoyée (service no-op)")

	return &service.EmailResult{
		MessageID: fmt.Sprintf("noop-%s@goshop.local", uuid.New().String()[:8]),
		SentAt:    time.Now(),
		Provider:  "noop",
	}, nil
}

// SendShopInvitation log l'invitation boutique sans l'envoyer
func (s *NoopEmailService) SendShopInvitation(data *service.ShopInvitationData) (*service.EmailResult, error) {
	if data == nil {
		return nil, service.ErrEmailInvalidRecipient
	}

	s.logger.Info().
		Str("to", data.RecipientEmail).
		Str("shop", data.ShopName).
		Str("role", data.Role).
		Str("role_display", data.RoleDisplayName).
		Str("inviter", data.InviterName).
		Str("invitation_id", data.InvitationID).
		Msg("📧 [NOOP] Invitation boutique non envoyée (service no-op)")

	return &service.EmailResult{
		MessageID: fmt.Sprintf("noop-%s@goshop.local", uuid.New().String()[:8]),
		SentAt:    time.Now(),
		Provider:  "noop",
	}, nil
}

// SendWelcome log l'email de bienvenue sans l'envoyer
func (s *NoopEmailService) SendWelcome(data *service.WelcomeData) (*service.EmailResult, error) {
	if data == nil {
		return nil, service.ErrEmailInvalidRecipient
	}

	s.logger.Info().
		Str("to", data.RecipientEmail).
		Str("name", data.RecipientName).
		Str("platform", data.PlatformName).
		Msg("📧 [NOOP] Email de bienvenue non envoyé (service no-op)")

	return &service.EmailResult{
		MessageID: fmt.Sprintf("noop-%s@goshop.local", uuid.New().String()[:8]),
		SentAt:    time.Now(),
		Provider:  "noop",
	}, nil
}

// SendSuspensionNotice log la notification de suspension sans l'envoyer
func (s *NoopEmailService) SendSuspensionNotice(data *service.SuspensionNoticeData) (*service.EmailResult, error) {
	if data == nil {
		return nil, service.ErrEmailInvalidRecipient
	}

	s.logger.Info().
		Str("to", data.ShopOwnerEmail).
		Str("shop", data.ShopName).
		Str("reason", data.Reason).
		Msg("📧 [NOOP] Notification suspension non envoyée (service no-op)")

	return &service.EmailResult{
		MessageID: fmt.Sprintf("noop-%s@goshop.local", uuid.New().String()[:8]),
		SentAt:    time.Now(),
		Provider:  "noop",
	}, nil
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// IsConfigured retourne false (service no-op)
func (s *NoopEmailService) IsConfigured() bool {
	return false
}

// GetProvider retourne "noop"
func (s *NoopEmailService) GetProvider() string {
	return "noop"
}

// ============================================================
// HELPERS
// ============================================================

// truncate tronque une chaîne à la longueur max
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
