package notification

import (
	"context"
	"fmt"

	"Goshop/domain/service"

	"github.com/rs/zerolog"
)

// NotificationProvider définit le contrat pour l'envoi de notifications
type NotificationProvider interface {
	// SendClientNotification envoie une notification à un client
	SendClientNotification(ctx context.Context, email, title, message string, data map[string]interface{}) error

	// SendMerchantNotification envoie une notification à un marchand
	SendMerchantNotification(ctx context.Context, email, title, message string, data map[string]interface{}) error

	// SendAdminNotification envoie une notification à l'administrateur
	SendAdminNotification(ctx context.Context, title, message string, data map[string]interface{}) error
}

// EmailNotificationProvider implémente NotificationProvider via SMTP
type EmailNotificationProvider struct {
	emailService service.EmailService
	platformName string
	platformURL  string
	adminEmail   string
	logger       zerolog.Logger
}

// NewEmailNotificationProvider crée une nouvelle instance du provider Email
func NewEmailNotificationProvider(
	emailService service.EmailService,
	platformName, platformURL, adminEmail string,
	logger zerolog.Logger,
) NotificationProvider {
	return &EmailNotificationProvider{
		emailService: emailService,
		platformName: platformName,
		platformURL:  platformURL,
		adminEmail:   adminEmail,
		logger:       logger.With().Str("component", "email_notification_provider").Logger(),
	}
}

// ============================================================
// MÉTHODES PUBLIQUES
// ============================================================

// SendClientNotification envoie une notification par email à un client
func (p *EmailNotificationProvider) SendClientNotification(ctx context.Context, email, title, message string, data map[string]interface{}) error {
	if email == "" {
		return fmt.Errorf("client email is empty")
	}

	htmlBody := p.buildNotificationHTML(title, message, data)
	textBody := p.buildNotificationText(title, message, data)

	msg := &service.EmailMessage{
		To:       email,
		ToName:   "Client",
		Subject:  fmt.Sprintf("🔔 %s - %s", p.platformName, title),
		HTMLBody: htmlBody,
		TextBody: textBody,
	}

	result, err := p.emailService.SendEmail(msg)
	if err != nil {
		p.logger.Error().Err(err).Str("email", email).Str("title", title).Msg("Failed to send client notification email")
		return fmt.Errorf("failed to send client notification: %w", err)
	}

	p.logger.Info().
		Str("message_id", result.MessageID).
		Str("email", email).
		Str("title", title).
		Msg("✅ Client notification email sent")

	return nil
}

// SendMerchantNotification envoie une notification par email à un marchand
func (p *EmailNotificationProvider) SendMerchantNotification(ctx context.Context, email, title, message string, data map[string]interface{}) error {
	if email == "" {
		return fmt.Errorf("merchant email is empty")
	}

	htmlBody := p.buildNotificationHTML(title, message, data)
	textBody := p.buildNotificationText(title, message, data)

	msg := &service.EmailMessage{
		To:       email,
		ToName:   "Marchand",
		Subject:  fmt.Sprintf("🏪 %s - %s", p.platformName, title),
		HTMLBody: htmlBody,
		TextBody: textBody,
	}

	result, err := p.emailService.SendEmail(msg)
	if err != nil {
		p.logger.Error().Err(err).Str("email", email).Str("title", title).Msg("Failed to send merchant notification email")
		return fmt.Errorf("failed to send merchant notification: %w", err)
	}

	p.logger.Info().
		Str("message_id", result.MessageID).
		Str("email", email).
		Str("title", title).
		Msg("✅ Merchant notification email sent")

	return nil
}

// SendAdminNotification envoie une notification par email à l'administrateur
func (p *EmailNotificationProvider) SendAdminNotification(ctx context.Context, title, message string, data map[string]interface{}) error {
	if p.adminEmail == "" {
		p.logger.Warn().Msg("Admin email not configured, skipping admin notification")
		return nil
	}

	htmlBody := p.buildNotificationHTML(title, message, data)
	textBody := p.buildNotificationText(title, message, data)

	msg := &service.EmailMessage{
		To:       p.adminEmail,
		ToName:   "Administrateur",
		Subject:  fmt.Sprintf("⚠️ [ADMIN] %s - %s", p.platformName, title),
		HTMLBody: htmlBody,
		TextBody: textBody,
	}

	result, err := p.emailService.SendEmail(msg)
	if err != nil {
		p.logger.Error().Err(err).Str("title", title).Msg("Failed to send admin notification email")
		return fmt.Errorf("failed to send admin notification: %w", err)
	}

	p.logger.Info().
		Str("message_id", result.MessageID).
		Str("title", title).
		Msg("✅ Admin notification email sent")

	return nil
}

// ============================================================
// MÉTHODES PRIVÉES - BUILDERS HTML/TEXT
// ============================================================

// buildNotificationHTML construit le corps HTML de la notification
func (p *EmailNotificationProvider) buildNotificationHTML(title, message string, data map[string]interface{}) string {
	detailsHTML := ""
	if len(data) > 0 {
		detailsHTML = `<div style="background-color: #f3f4f6; padding: 15px; border-radius: 6px; margin: 20px 0;">
			<p style="margin: 5px 0;"><strong>Détails :</strong></p>
			<ul style="margin: 5px 0; padding-left: 20px;">`

		for key, value := range data {
			detailsHTML += fmt.Sprintf(`<li style="margin: 3px 0;"><strong>%s :</strong> %v</li>`, key, value)
		}

		detailsHTML += `</ul></div>`
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>%s</title>
</head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; background-color: #f9fafb; padding: 20px;">
    <div style="background-color: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
        <h1 style="color: #2563eb; margin-top: 0; font-size: 24px;">%s</h1>
        
        <p style="font-size: 16px; line-height: 1.6; color: #374151;">%s</p>
        
        %s
        
        <div style="text-align: center; margin: 30px 0;">
            <a href="%s" style="display: inline-block; padding: 14px 28px; background-color: #2563eb; color: white; text-decoration: none; border-radius: 6px; font-weight: bold;">
                Accéder à mon compte
            </a>
        </div>
        
        <hr style="border: none; border-top: 1px solid #e5e7eb; margin: 30px 0;">
        
        <p style="color: #6b7280; font-size: 12px; text-align: center;">
            © 2026 %s. Tous droits réservés.<br>
            Vous recevez cet email car vous êtes inscrit sur %s.
        </p>
    </div>
</body>
</html>`,
		title,
		title,
		message,
		detailsHTML,
		p.platformURL,
		p.platformName,
		p.platformName,
	)
}

// buildNotificationText construit la version texte de la notification
func (p *EmailNotificationProvider) buildNotificationText(title, message string, data map[string]interface{}) string {
	text := fmt.Sprintf(`%s

%s

`, title, message)

	if len(data) > 0 {
		text += "Détails :\n"
		for key, value := range data {
			text += fmt.Sprintf("- %s : %v\n", key, value)
		}
		text += "\n"
	}

	text += fmt.Sprintf(`Pour accéder à votre compte : %s

© 2026 %s. Tous droits réservés.`, p.platformURL, p.platformName)

	return text
}

// ============================================================
// HELPERS SPÉCIFIQUES AUX TYPES DE NOTIFICATIONS
// ============================================================

// NotifyDisputeResolved envoie une notification de résolution de litige
func (p *EmailNotificationProvider) NotifyDisputeResolved(ctx context.Context, email, orderID, resolution string) error {
	title := "Litige résolu"
	message := fmt.Sprintf("Votre litige concernant la commande #%s a été traité. Statut : %s.", orderID, resolution)
	data := map[string]interface{}{
		"order_id":   orderID,
		"resolution": resolution,
	}

	return p.SendClientNotification(ctx, email, title, message, data)
}

// NotifyOrderDelivered envoie une notification de livraison
func (p *EmailNotificationProvider) NotifyOrderDelivered(ctx context.Context, email, orderID string, amount int64) error {
	title := "Commande livrée"
	message := fmt.Sprintf("Votre commande #%s a été livrée avec succès. Montant reçu : %d FCFA", orderID, amount/100)
	data := map[string]interface{}{
		"order_id": orderID,
		"amount":   fmt.Sprintf("%d FCFA", amount/100),
	}

	return p.SendClientNotification(ctx, email, title, message, data)
}

// NotifyWithdrawalCompleted envoie une notification de retrait complété
func (p *EmailNotificationProvider) NotifyWithdrawalCompleted(ctx context.Context, email, withdrawalID string, amount int64) error {
	title := "Retrait effectué"
	message := fmt.Sprintf("Votre retrait de %d FCFA a été effectué avec succès (ID: %s).", amount/100, withdrawalID)
	data := map[string]interface{}{
		"withdrawal_id": withdrawalID,
		"amount":        fmt.Sprintf("%d FCFA", amount/100),
	}

	return p.SendMerchantNotification(ctx, email, title, message, data)
}

// NotifyKYCApproved envoie une notification d'approbation KYC
func (p *EmailNotificationProvider) NotifyKYCApproved(ctx context.Context, email string) error {
	title := "KYC approuvé"
	message := "Votre vérification d'identité a été approuvée. Vous pouvez maintenant utiliser toutes les fonctionnalités de la plateforme."
	data := map[string]interface{}{
		"status": "approuvé",
	}

	return p.SendClientNotification(ctx, email, title, message, data)
}
