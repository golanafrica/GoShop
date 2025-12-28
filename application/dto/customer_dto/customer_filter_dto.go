// application/dto/customer_filter_dto.go
package dto

// CustomerFilter représente les critères optionnels pour filtrer la liste des clients.
// Les champs sont des pointeurs pour permettre de distinguer "non fourni" de "valeur vide".
type CustomerFilter struct {
	Name  *string `json:"name,omitempty"`  // filtre partiel sur prénom OU nom
	Email *string `json:"email,omitempty"` // filtre exact ou partiel sur email
}
