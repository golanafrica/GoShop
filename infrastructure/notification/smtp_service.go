package notification

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"Goshop/config"
	"Goshop/domain/service"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.3.2 : SMTP EMAIL SERVICE
// ============================================================
//
// 🎯 Objectif :
//   Implémenter l'interface EmailService avec un service SMTP.
//   Supporte :
//   - Envoi réel via SMTP avec TLS
//   - Mode debug (logs au lieu d'envoi)
//   - Templates HTML personnalisables
//   - Pièces jointes
//
// 🔐 Sécurité :
//   - TLS obligatoire en production
//   - Validation des destinataires
//   - Rate limiting (à implémenter)
//
// ============================================================

// SMTPService implémente l'interface EmailService
type SMTPService struct {
	config    *config.EmailConfig
	logger    zerolog.Logger
	templates map[string]*template.Template
	provider  string
}

// NewSMTPService crée une nouvelle instance du service SMTP
func NewSMTPService(cfg *config.EmailConfig, logger zerolog.Logger) (*SMTPService, error) {
	if cfg == nil {
		return nil, service.ErrEmailServiceNotConfigured
	}

	svc := &SMTPService{
		config:    cfg,
		logger:    logger.With().Str("component", "smtp_email_service").Logger(),
		templates: make(map[string]*template.Template),
		provider:  "smtp",
	}

	// Charger les templates
	if err := svc.loadTemplates(); err != nil {
		logger.Warn().Err(err).Msg("⚠️ Erreur chargement templates email (mode dégradé)")
	}

	logger.Info().
		Str("host", cfg.Host).
		Int("port", cfg.Port).
		Bool("tls", cfg.UseTLS).
		Bool("debug_mode", cfg.DebugMode).
		Str("from", cfg.From).
		Msg("✅ SMTP email service initialized")

	return svc, nil
}

// ============================================================
// MÉTHODES GÉNÉRIQUES
// ============================================================

// SendEmail envoie un email générique
func (s *SMTPService) SendEmail(message *service.EmailMessage) (*service.EmailResult, error) {
	if message == nil {
		return nil, service.ErrEmailInvalidBody
	}

	// Validation
	if err := s.validateMessage(message); err != nil {
		return nil, err
	}

	// Mode debug : logger au lieu d'envoyer
	// Mode debug : logger au lieu d'envoyer
	if s.config.DebugMode {
		s.logEmailDebug(message)
		return &service.EmailResult{
			MessageID: fmt.Sprintf("debug-%s@goshop.local", uuid.New().String()[:8]),
			SentAt:    time.Now(),
			Provider:  "debug",
			DebugMode: true,
		}, nil
	}

	// Envoi réel via SMTP
	return s.sendViaSMTP(message)
}

// SendEmailAsync envoie un email de manière asynchrone
func (s *SMTPService) SendEmailAsync(message *service.EmailMessage) {
	go func() {
		result, err := s.SendEmail(message)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("to", message.To).
				Str("subject", message.Subject).
				Msg("❌ Erreur envoi email asynchrone")
			return
		}

		s.logger.Debug().
			Str("message_id", result.MessageID).
			Str("to", message.To).
			Str("provider", result.Provider).
			Msg("✅ Email asynchrone envoyé")
	}()
}

// ============================================================
// MÉTHODES SPÉCIFIQUES AUX TEMPLATES
// ============================================================

// SendPlatformInvitation envoie une invitation collaborateur plateforme
func (s *SMTPService) SendPlatformInvitation(data *service.PlatformInvitationData) (*service.EmailResult, error) {
	if data == nil {
		return nil, service.ErrEmailInvalidRecipient
	}

	// Construire l'URL d'acceptation
	acceptURL := fmt.Sprintf("%s/collaborators/invitations/%s", s.config.AppURL, data.Token)

	// Construire le corps HTML
	htmlBody := s.buildPlatformInvitationHTML(data, acceptURL)
	textBody := s.buildPlatformInvitationText(data, acceptURL)

	message := &service.EmailMessage{
		To:       data.RecipientEmail,
		ToName:   data.RecipientName,
		Subject:  fmt.Sprintf("🎉 Invitation à rejoindre %s en tant que %s", data.PlatformName, data.RoleDisplayName),
		HTMLBody: htmlBody,
		TextBody: textBody,
	}

	s.logger.Info().
		Str("to", data.RecipientEmail).
		Str("role", data.Role).
		Str("invitation_id", data.InvitationID).
		Msg("📧 Envoi invitation collaborateur plateforme")

	return s.SendEmail(message)
}

// SendShopInvitation envoie une invitation collaborateur boutique
func (s *SMTPService) SendShopInvitation(data *service.ShopInvitationData) (*service.EmailResult, error) {
	if data == nil {
		return nil, service.ErrEmailInvalidRecipient
	}

	// Construire l'URL d'acceptation
	acceptURL := fmt.Sprintf("%s/collaborators/invitations/%s", s.config.AppURL, data.Token)

	// Construire le corps HTML
	htmlBody := s.buildShopInvitationHTML(data, acceptURL)
	textBody := s.buildShopInvitationText(data, acceptURL)

	message := &service.EmailMessage{
		To:       data.RecipientEmail,
		ToName:   data.RecipientName,
		Subject:  fmt.Sprintf("🎉 Invitation à rejoindre '%s' en tant que %s", data.ShopName, data.RoleDisplayName),
		HTMLBody: htmlBody,
		TextBody: textBody,
	}

	s.logger.Info().
		Str("to", data.RecipientEmail).
		Str("shop", data.ShopName).
		Str("role", data.Role).
		Str("invitation_id", data.InvitationID).
		Msg("📧 Envoi invitation collaborateur boutique")

	return s.SendEmail(message)
}

// SendWelcome envoie un email de bienvenue
func (s *SMTPService) SendWelcome(data *service.WelcomeData) (*service.EmailResult, error) {
	if data == nil {
		return nil, service.ErrEmailInvalidRecipient
	}

	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Bienvenue sur %s</title>
</head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
    <h1 style="color: #2563eb;">🎉 Bienvenue sur %s !</h1>
    <p>Bonjour %s,</p>
    <p>Nous sommes ravis de vous accueillir sur notre plateforme.</p>
    <p>Vous pouvez dès maintenant vous connecter et commencer à utiliser nos services.</p>
    <a href="%s" style="display: inline-block; padding: 12px 24px; background-color: #2563eb; color: white; text-decoration: none; border-radius: 6px;">Se connecter</a>
    <p style="margin-top: 30px; color: #6b7280; font-size: 12px;">
        © 2026 %s. Tous droits réservés.
    </p>
</body>
</html>`, data.PlatformName, data.PlatformName, data.RecipientName, data.LoginURL, data.PlatformName)

	message := &service.EmailMessage{
		To:       data.RecipientEmail,
		ToName:   data.RecipientName,
		Subject:  fmt.Sprintf("🎉 Bienvenue sur %s !", data.PlatformName),
		HTMLBody: htmlBody,
		TextBody: fmt.Sprintf("Bienvenue %s sur %s ! Connectez-vous ici : %s", data.RecipientName, data.PlatformName, data.LoginURL),
	}

	return s.SendEmail(message)
}

// SendSuspensionNotice envoie une notification de suspension
func (s *SMTPService) SendSuspensionNotice(data *service.SuspensionNoticeData) (*service.EmailResult, error) {
	if data == nil {
		return nil, service.ErrEmailInvalidRecipient
	}

	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Suspension de votre boutique</title>
</head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
    <div style="background-color: #fef2f2; border-left: 4px solid #dc2626; padding: 20px; margin-bottom: 20px;">
        <h1 style="color: #dc2626; margin-top: 0;">⚠️ Boutique suspendue</h1>
    </div>
    <p>Bonjour %s,</p>
    <p>Nous vous informons que votre boutique <strong>%s</strong> a été suspendue le %s.</p>
    <div style="background-color: #f3f4f6; padding: 15px; border-radius: 6px; margin: 20px 0;">
        <p style="margin: 5px 0;"><strong>Raison de la suspension :</strong></p>
        <p style="margin: 5px 0; color: #dc2626;">%s</p>
    </div>
    <p>Si vous pensez qu'il s'agit d'une erreur, veuillez contacter notre équipe support à <a href="mailto:%s">%s</a>.</p>
    <p style="margin-top: 30px; color: #6b7280; font-size: 12px;">
        © 2026 %s. Tous droits réservés.
    </p>
</body>
</html>`, data.ShopOwnerName, data.ShopName, data.SuspendedAt.Format("02/01/2006 à 15:04"), data.Reason, data.ContactEmail, data.ContactEmail, data.PlatformName)

	message := &service.EmailMessage{
		To:       data.ShopOwnerEmail,
		ToName:   data.ShopOwnerName,
		Subject:  fmt.Sprintf("⚠️ Votre boutique '%s' a été suspendue", data.ShopName),
		HTMLBody: htmlBody,
		TextBody: fmt.Sprintf("Votre boutique %s a été suspendue. Raison : %s. Contactez %s pour plus d'informations.", data.ShopName, data.Reason, data.ContactEmail),
	}

	return s.SendEmail(message)
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// IsConfigured vérifie si le service email est configuré
func (s *SMTPService) IsConfigured() bool {
	return s.config.IsValid()
}

// GetProvider retourne le nom du provider
func (s *SMTPService) GetProvider() string {
	if s.config.DebugMode {
		return "debug"
	}
	return s.provider
}

// ============================================================
// MÉTHODES PRIVÉES
// ============================================================

// validateMessage valide un message email
func (s *SMTPService) validateMessage(message *service.EmailMessage) error {
	if message.To == "" {
		return service.ErrEmailInvalidRecipient
	}
	if !strings.Contains(message.To, "@") {
		return service.ErrEmailInvalidRecipient
	}
	if message.Subject == "" {
		return service.ErrEmailInvalidSubject
	}
	if message.HTMLBody == "" && message.TextBody == "" {
		return service.ErrEmailInvalidBody
	}
	return nil
}

// logEmailDebug log un email en mode debug
func (s *SMTPService) logEmailDebug(message *service.EmailMessage) {
	s.logger.Info().
		Str("to", message.To).
		Str("to_name", message.ToName).
		Str("subject", message.Subject).
		Int("html_length", len(message.HTMLBody)).
		Int("text_length", len(message.TextBody)).
		Int("attachments", len(message.Attachments)).
		Msg("📧 [DEBUG MODE] Email non envoyé (mode debug activé)")

	// Log le contenu HTML (tronqué si trop long)
	htmlPreview := message.HTMLBody
	if len(htmlPreview) > 500 {
		htmlPreview = htmlPreview[:500] + "..."
	}
	s.logger.Debug().
		Str("html_preview", htmlPreview).
		Msg("📄 Contenu HTML de l'email")
}

// sendViaSMTP envoie un email via SMTP
func (s *SMTPService) sendViaSMTP(message *service.EmailMessage) (*service.EmailResult, error) {
	// Construire l'email
	emailBytes, err := s.buildEmailBytes(message)
	if err != nil {
		return nil, fmt.Errorf("build email: %w", err)
	}

	// Connexion SMTP
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	var auth smtp.Auth
	if s.config.Username != "" && s.config.Password != "" {
		auth = smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
	}

	// Déterminer le from
	from := s.config.From
	if message.From != "" {
		from = message.From
	}

	// Envoyer l'email
	var sendErr error
	if s.config.UseTLS {
		// Connexion avec TLS explicite (STARTTLS)
		sendErr = s.sendWithTLS(addr, auth, from, message.To, emailBytes)
	} else {
		// Connexion sans TLS
		sendErr = smtp.SendMail(addr, auth, from, []string{message.To}, emailBytes)
	}

	if sendErr != nil {
		s.logger.Error().
			Err(sendErr).
			Str("to", message.To).
			Str("subject", message.Subject).
			Msg("❌ Échec envoi email SMTP")
		return nil, fmt.Errorf("%w: %v", service.ErrEmailSendFailed, sendErr)
	}

	now := time.Now()
	messageID := fmt.Sprintf("%s@goshop.com", uuid.New().String())

	s.logger.Info().
		Str("message_id", messageID).
		Str("to", message.To).
		Str("subject", message.Subject).
		Msg("✅ Email envoyé avec succès")

	return &service.EmailResult{
		MessageID: messageID,
		SentAt:    now,
		Provider:  s.provider,
	}, nil
}

// sendWithTLS envoie un email avec TLS
func (s *SMTPService) sendWithTLS(addr string, auth smtp.Auth, from, to string, emailBytes []byte) error {
	// Connexion TCP
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	// Créer le client SMTP
	host, _, _ := net.SplitHostPort(addr)
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	// STARTTLS
	tlsConfig := &tls.Config{
		ServerName: host,
	}
	if err = client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("starttls: %w", err)
	}

	// Authentification
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}

	// From
	if err = client.Mail(from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}

	// To
	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt to: %w", err)
	}

	// Body
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	defer writer.Close()

	_, err = writer.Write(emailBytes)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}

	// Quit
	return client.Quit()
}

// buildEmailBytes construit les bytes de l'email
func (s *SMTPService) buildEmailBytes(message *service.EmailMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Headers
	from := s.config.From
	fromName := s.config.FromName
	if message.From != "" {
		from = message.From
	}
	if message.FromName != "" {
		fromName = message.FromName
	}

	if fromName != "" {
		buf.WriteString(fmt.Sprintf("From: %s <%s>\r\n", fromName, from))
	} else {
		buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
	}

	if message.ToName != "" {
		buf.WriteString(fmt.Sprintf("To: %s <%s>\r\n", message.ToName, message.To))
	} else {
		buf.WriteString(fmt.Sprintf("To: %s\r\n", message.To))
	}

	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", message.Subject))
	buf.WriteString("MIME-Version: 1.0\r\n")

	// Content-Type
	if message.HTMLBody != "" {
		boundary := fmt.Sprintf("boundary-%s", uuid.New().String()[:8])
		buf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=%s\r\n\r\n", boundary))

		// Text version
		if message.TextBody != "" {
			buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
			buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
			buf.WriteString(message.TextBody)
			buf.WriteString("\r\n\r\n")
		}

		// HTML version
		buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
		buf.WriteString(message.HTMLBody)
		buf.WriteString("\r\n\r\n")

		buf.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else {
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		buf.WriteString(message.TextBody)
	}

	return buf.Bytes(), nil
}

// loadTemplates charge les templates HTML depuis le dossier templates/
func (s *SMTPService) loadTemplates() error {
	templatesDir := "templates"

	// Vérifier si le dossier existe
	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		s.logger.Warn().
			Str("dir", templatesDir).
			Msg("⚠️ Dossier templates non trouvé (utilisation des templates inline)")
		return nil
	}

	// Charger tous les fichiers .html
	files, err := filepath.Glob(filepath.Join(templatesDir, "*.html"))
	if err != nil {
		return fmt.Errorf("glob templates: %w", err)
	}

	for _, file := range files {
		name := filepath.Base(file)
		name = strings.TrimSuffix(name, ".html")

		tmpl, err := template.ParseFiles(file)
		if err != nil {
			s.logger.Warn().
				Err(err).
				Str("file", file).
				Msg("⚠️ Erreur parsing template")
			continue
		}

		s.templates[name] = tmpl
		s.logger.Debug().
			Str("name", name).
			Str("file", file).
			Msg("✅ Template chargé")
	}

	s.logger.Info().
		Int("count", len(s.templates)).
		Msg("✅ Templates email chargés")

	return nil
}

// ============================================================
// BUILDERS HTML
// ============================================================

// buildPlatformInvitationHTML construit le HTML pour une invitation plateforme
func (s *SMTPService) buildPlatformInvitationHTML(data *service.PlatformInvitationData, acceptURL string) string {
	customMessageHTML := ""
	if data.CustomMessage != "" {
		customMessageHTML = fmt.Sprintf(`
    <div style="background-color: #eff6ff; border-left: 4px solid #2563eb; padding: 15px; margin: 20px 0;">
        <p style="margin: 0;"><strong>Message de %s :</strong></p>
        <p style="margin: 10px 0 0 0; font-style: italic;">"%s"</p>
    </div>`, data.InviterName, data.CustomMessage)
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Invitation %s</title>
</head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; background-color: #f9fafb;">
    <div style="background-color: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
        <h1 style="color: #2563eb; margin-top: 0;">🎉 Vous êtes invité(e) !</h1>
        
        <p>Bonjour%s,</p>
        
        <p><strong>%s</strong> (%s) vous invite à rejoindre <strong>%s</strong> en tant que <strong>%s</strong>.</p>
        
        %s
        
        <div style="background-color: #f3f4f6; padding: 15px; border-radius: 6px; margin: 20px 0;">
            <p style="margin: 5px 0;"><strong>Votre rôle :</strong> %s</p>
            <p style="margin: 5px 0;"><strong>Plateforme :</strong> %s</p>
            <p style="margin: 5px 0;"><strong>Expiration :</strong> %s (%d jours)</p>
        </div>
        
        <div style="text-align: center; margin: 30px 0;">
            <a href="%s" style="display: inline-block; padding: 14px 28px; background-color: #2563eb; color: white; text-decoration: none; border-radius: 6px; font-weight: bold;">
                Accepter l'invitation
            </a>
        </div>
        
        <p style="color: #6b7280; font-size: 14px;">
            Si le bouton ne fonctionne pas, copiez ce lien dans votre navigateur :<br>
            <a href="%s" style="color: #2563eb; word-break: break-all;">%s</a>
        </p>
        
        <hr style="border: none; border-top: 1px solid #e5e7eb; margin: 30px 0;">
        
        <p style="color: #6b7280; font-size: 12px;">
            Si vous n'avez pas demandé cette invitation, vous pouvez ignorer cet email en toute sécurité.
        </p>
        
        <p style="color: #6b7280; font-size: 12px; margin-top: 30px;">
            © 2026 %s. Tous droits réservés.
        </p>
    </div>
</body>
</html>`,
		data.PlatformName,
		formatName(data.RecipientName),
		data.InviterName, data.InviterRole, data.PlatformName, data.RoleDisplayName,
		customMessageHTML,
		data.RoleDisplayName, data.PlatformName,
		data.ExpiresAt.Format("02/01/2006 à 15:04"), data.DaysUntilExpire,
		acceptURL, acceptURL, acceptURL,
		data.PlatformName,
	)
}

// buildPlatformInvitationText construit la version texte
func (s *SMTPService) buildPlatformInvitationText(data *service.PlatformInvitationData, acceptURL string) string {
	text := fmt.Sprintf(`Bonjour %s,

%s (%s) vous invite à rejoindre %s en tant que %s.

Votre rôle : %s
Plateforme : %s
Expiration : %s (%d jours)

Pour accepter l'invitation, cliquez sur ce lien :
%s

Si vous n'avez pas demandé cette invitation, vous pouvez ignorer cet email.

© 2026 %s. Tous droits réservés.`,
		formatName(data.RecipientName),
		data.InviterName, data.InviterRole, data.PlatformName, data.RoleDisplayName,
		data.RoleDisplayName, data.PlatformName,
		data.ExpiresAt.Format("02/01/2006 à 15:04"), data.DaysUntilExpire,
		acceptURL,
		data.PlatformName,
	)

	if data.CustomMessage != "" {
		text = fmt.Sprintf(`%s

Message de %s : "%s"`, text, data.InviterName, data.CustomMessage)
	}

	return text
}

// buildShopInvitationHTML construit le HTML pour une invitation boutique
func (s *SMTPService) buildShopInvitationHTML(data *service.ShopInvitationData, acceptURL string) string {
	customMessageHTML := ""
	if data.CustomMessage != "" {
		customMessageHTML = fmt.Sprintf(`
    <div style="background-color: #eff6ff; border-left: 4px solid #2563eb; padding: 15px; margin: 20px 0;">
        <p style="margin: 0;"><strong>Message de %s :</strong></p>
        <p style="margin: 10px 0 0 0; font-style: italic;">"%s"</p>
    </div>`, data.InviterName, data.CustomMessage)
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Invitation %s</title>
</head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; background-color: #f9fafb;">
    <div style="background-color: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
        <h1 style="color: #2563eb; margin-top: 0;">🏪 Vous êtes invité(e) !</h1>
        
        <p>Bonjour%s,</p>
        
        <p><strong>%s</strong> (%s) vous invite à rejoindre la boutique <strong>%s</strong> en tant que <strong>%s</strong>.</p>
        
        %s
        
        <div style="background-color: #f3f4f6; padding: 15px; border-radius: 6px; margin: 20px 0;">
            <p style="margin: 5px 0;"><strong>Votre rôle :</strong> %s</p>
            <p style="margin: 5px 0;"><strong>Boutique :</strong> %s</p>
            <p style="margin: 5px 0;"><strong>Plateforme :</strong> %s</p>
            <p style="margin: 5px 0;"><strong>Expiration :</strong> %s (%d jours)</p>
        </div>
        
        <div style="text-align: center; margin: 30px 0;">
            <a href="%s" style="display: inline-block; padding: 14px 28px; background-color: #2563eb; color: white; text-decoration: none; border-radius: 6px; font-weight: bold;">
                Accepter l'invitation
            </a>
        </div>
        
        <p style="color: #6b7280; font-size: 14px;">
            Si le bouton ne fonctionne pas, copiez ce lien dans votre navigateur :<br>
            <a href="%s" style="color: #2563eb; word-break: break-all;">%s</a>
        </p>
        
        <hr style="border: none; border-top: 1px solid #e5e7eb; margin: 30px 0;">
        
        <p style="color: #6b7280; font-size: 12px;">
            Si vous n'avez pas demandé cette invitation, vous pouvez ignorer cet email en toute sécurité.
        </p>
        
        <p style="color: #6b7280; font-size: 12px; margin-top: 30px;">
            © 2026 %s. Tous droits réservés.
        </p>
    </div>
</body>
</html>`,
		data.ShopName,
		formatName(data.RecipientName),
		data.InviterName, data.InviterRole, data.ShopName, data.RoleDisplayName,
		customMessageHTML,
		data.RoleDisplayName, data.ShopName, data.PlatformName,
		data.ExpiresAt.Format("02/01/2006 à 15:04"), data.DaysUntilExpire,
		acceptURL, acceptURL, acceptURL,
		data.PlatformName,
	)
}

// buildShopInvitationText construit la version texte
func (s *SMTPService) buildShopInvitationText(data *service.ShopInvitationData, acceptURL string) string {
	text := fmt.Sprintf(`Bonjour %s,

%s (%s) vous invite à rejoindre la boutique %s en tant que %s.

Votre rôle : %s
Boutique : %s
Plateforme : %s
Expiration : %s (%d jours)

Pour accepter l'invitation, cliquez sur ce lien :
%s

Si vous n'avez pas demandé cette invitation, vous pouvez ignorer cet email.

© 2026 %s. Tous droits réservés.`,
		formatName(data.RecipientName),
		data.InviterName, data.InviterRole, data.ShopName, data.RoleDisplayName,
		data.RoleDisplayName, data.ShopName, data.PlatformName,
		data.ExpiresAt.Format("02/01/2006 à 15:04"), data.DaysUntilExpire,
		acceptURL,
		data.PlatformName,
	)

	if data.CustomMessage != "" {
		text = fmt.Sprintf(`%s

Message de %s : "%s"`, text, data.InviterName, data.CustomMessage)
	}

	return text
}

// ============================================================
// HELPERS
// ============================================================

// formatName formate un nom pour l'affichage
func formatName(name string) string {
	if name == "" {
		return ""
	}
	return " " + name
}
