package service

//go:generate mockgen -destination=../../mocks/service/mock_email_service.go -package=service . EmailService

import (
	"errors"
	"time"
)

// ============================================================
// 🆕 v4.3.2 : EMAIL SERVICE INTERFACE
// ============================================================

// ============================================================
// ERREURS
// ============================================================

var (
	ErrEmailServiceNotConfigured = errors.New("email service not configured")
	ErrEmailInvalidRecipient     = errors.New("invalid email recipient")
	ErrEmailInvalidSubject       = errors.New("invalid email subject")
	ErrEmailInvalidBody          = errors.New("invalid email body")
	ErrEmailSendFailed           = errors.New("failed to send email")
	ErrEmailTemplateNotFound     = errors.New("email template not found")
)

// ============================================================
// STRUCTURES DE DONNÉES
// ============================================================

// EmailMessage représente un email générique
type EmailMessage struct {
	To          string
	ToName      string
	Subject     string
	HTMLBody    string
	TextBody    string
	From        string
	FromName    string
	ReplyTo     string
	Attachments []EmailAttachment
}

// EmailAttachment représente une pièce jointe
type EmailAttachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// EmailResult représente le résultat d'un envoi d'email
type EmailResult struct {
	MessageID   string
	SentAt      time.Time
	DeliveredAt *time.Time
	Provider    string // "smtp", "sendgrid", "mailgun", "debug", "noop"
	DebugMode   bool   // 🆕 v4.3.2 : true si mode debug (email loggé, pas envoyé)
}

// ============================================================
// DONNÉES SPÉCIFIQUES AUX TEMPLATES
// ============================================================

// PlatformInvitationData représente les données pour une invitation plateforme
type PlatformInvitationData struct {
	RecipientEmail  string
	RecipientName   string
	InvitationID    string
	Token           string
	Role            string
	RoleDisplayName string
	InviterName     string
	InviterEmail    string
	InviterRole     string
	PlatformName    string
	PlatformURL     string
	CustomMessage   string
	ExpiresAt       time.Time
	DaysUntilExpire int
}

// ShopInvitationData représente les données pour une invitation boutique
type ShopInvitationData struct {
	RecipientEmail  string
	RecipientName   string
	InvitationID    string
	Token           string
	Role            string
	RoleDisplayName string
	ShopID          string
	ShopName        string
	ShopSlug        string
	InviterName     string
	InviterEmail    string
	InviterRole     string
	PlatformName    string
	PlatformURL     string
	CustomMessage   string
	ExpiresAt       time.Time
	DaysUntilExpire int
}

// WelcomeData représente les données pour un email de bienvenue
type WelcomeData struct {
	RecipientEmail string
	RecipientName  string
	PlatformName   string
	PlatformURL    string
	LoginURL       string
}

// SuspensionNoticeData représente les données pour une notification de suspension
type SuspensionNoticeData struct {
	ShopOwnerEmail string
	ShopOwnerName  string
	ShopName       string
	Reason         string
	SuspendedAt    time.Time
	SuspendedBy    string
	PlatformName   string
	PlatformURL    string
	ContactEmail   string
}

// ============================================================
// INTERFACE PRINCIPALE
// ============================================================

// EmailService définit les opérations d'envoi d'emails
type EmailService interface {
	// SendEmail envoie un email générique
	SendEmail(message *EmailMessage) (*EmailResult, error)

	// SendEmailAsync envoie un email de manière asynchrone (via goroutine)
	SendEmailAsync(message *EmailMessage)

	// SendPlatformInvitation envoie une invitation collaborateur plateforme
	SendPlatformInvitation(data *PlatformInvitationData) (*EmailResult, error)

	// SendShopInvitation envoie une invitation collaborateur boutique
	SendShopInvitation(data *ShopInvitationData) (*EmailResult, error)

	// SendWelcome envoie un email de bienvenue
	SendWelcome(data *WelcomeData) (*EmailResult, error)

	// SendSuspensionNotice envoie une notification de suspension de boutique
	SendSuspensionNotice(data *SuspensionNoticeData) (*EmailResult, error)

	// IsConfigured vérifie si le service email est configuré
	IsConfigured() bool

	// GetProvider retourne le nom du provider (smtp, sendgrid, etc.)
	GetProvider() string
}

// ============================================================
// HELPERS POUR LES RÔLES
// ============================================================

// GetPlatformRoleDisplayName retourne le nom affiché d'un rôle plateforme
func GetPlatformRoleDisplayName(role string) string {
	roleNames := map[string]string{
		"finance_manager":   "Responsable Finances",
		"support_manager":   "Responsable Support",
		"kyc_reviewer":      "Validateur KYC",
		"marketing_manager": "Responsable Marketing",
		"tech_admin":        "Administrateur Technique",
	}

	if name, ok := roleNames[role]; ok {
		return name
	}
	return role
}

// GetShopRoleDisplayName retourne le nom affiché d'un rôle boutique
func GetShopRoleDisplayName(role string) string {
	roleNames := map[string]string{
		"shop_admin": "Administrateur Boutique",
		"seller":     "Vendeur",
		"support":    "Support Client",
		"accountant": "Comptable",
	}

	if name, ok := roleNames[role]; ok {
		return name
	}
	return role
}
