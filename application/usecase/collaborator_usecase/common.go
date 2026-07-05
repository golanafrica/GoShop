package collaboratorusecase

// ============================================================
// 🆕 v4.3.0 : FICHIER COMMUN - HELPERS PARTAGÉS
// ============================================================
//
// 🎯 Objectif :
//   Contient les types et helpers partagés entre tous les usecases
//   du package collaborator_usecase.
//
// 📋 Contenu :
//   - AdminContext : Informations de l'admin effectuant l'action
//   - strPtr : Helper pour créer des pointeurs string
//
// ============================================================

// AdminContext contient les informations de l'admin qui effectue l'action
type AdminContext struct {
	AdminID    string `json:"admin_id"`
	AdminEmail string `json:"admin_email"`
	AdminRole  string `json:"admin_role"`
	IPAddress  string `json:"ip_address"`
	UserAgent  string `json:"user_agent"`
	RequestID  string `json:"request_id"`
}

// ============================================================
// 🆕 v4.3.0 : HELPERS PARTAGÉS
// ============================================================

// strPtr crée un pointeur vers une string
// Utilisé pour les champs optionnels dans les réponses JSON
func strPtr(s string) *string {
	return &s
}

// boolPtr crée un pointeur vers un bool
func boolPtr(b bool) *bool {
	return &b
}

// intPtr crée un pointeur vers un int
func intPtr(i int) *int {
	return &i
}
