package utils

import (
	"net/http"
)

// JWT / Auth errors
var (
	ErrAccessTokenExpired = NewAppError(
		"ACCESS_TOKEN_EXPIRED",
		"access token expired",
		http.StatusUnauthorized,
	)

	ErrRefreshTokenExpired = NewAppError(
		"REFRESH_TOKEN_EXPIRED",
		"refresh token expired",
		http.StatusUnauthorized,
	)

	ErrRefreshTokenInvalid = NewAppError(
		"REFRESH_TOKEN_INVALID",
		"invalid refresh token",
		http.StatusUnauthorized,
	)

	ErrRefreshTokenRevoked = NewAppError(
		"REFRESH_TOKEN_REVOKED",
		"refresh token has been revoked",
		http.StatusUnauthorized,
	)

	ErrRefreshTokenNotFound = NewAppError(
		"REFRESH_TOKEN_NOT_FOUND",
		"refresh token not found",
		http.StatusUnauthorized,
	)

	ErrTokenMalformed = NewAppError(
		"TOKEN_MALFORMED",
		"invalid or corrupted token",
		http.StatusUnauthorized,
	)

	ErrTokenSignatureInvalid = NewAppError(
		"TOKEN_SIGNATURE_INVALID",
		"invalid token signature",
		http.StatusUnauthorized,
	)

	ErrTokenMissing = NewAppError(
		"TOKEN_MISSING",
		"missing Authorization header",
		http.StatusUnauthorized,
	)

	ErrTokenTypeInvalid = NewAppError(
		"TOKEN_TYPE_INVALID",
		"invalid token type",
		http.StatusUnauthorized,
	)

	ErrTokenJTIInvalid = NewAppError(
		"TOKEN_JTI_INVALID",
		"token jti invalid or missing",
		http.StatusUnauthorized,
	)

	ErrTokenFormatInvalid = NewAppError(
		"TOKEN_FORMAT_INVALID",
		"invalid token format",
		http.StatusUnauthorized,
	)

	ErrTokenSubjectInvalid = NewAppError(
		"TOKEN_SUBJECT_INVALID",
		"invalid token subject",
		http.StatusUnauthorized,
	)
)

var (
	// Erreurs générales
	ErrInvalidPayload   = NewAppError("INVALID_PAYLOAD", "invalid request body", http.StatusBadRequest)
	ErrValidationFailed = NewAppError("VALIDATION_FAILED", "invalid fields in request", http.StatusBadRequest)
	ErrInternalServer   = NewAppError("INTERNAL_SERVER_ERROR", "unexpected server error", http.StatusInternalServerError)
	ErrNotFound         = NewAppError("NOT_FOUND", "resource not found", http.StatusNotFound)
	ErrUnauthorized     = NewAppError("UNAUTHORIZED", "unauthorized", http.StatusUnauthorized)
	ErrForbidden        = NewAppError("FORBIDDEN", "forbidden", http.StatusForbidden)

	// Customer errors
	ErrCustomerNotFound   = NewAppError("CUSTOMER_NOT_FOUND", "customer not found", http.StatusNotFound)
	ErrCustomerCreateFail = NewAppError("CUSTOMER_CREATION_FAILED", "unable to create customer", http.StatusInternalServerError)
	ErrCustomerUpdateFail = NewAppError("CUSTOMER_UPDATE_FAILED", "unable to update customer", http.StatusInternalServerError)
	ErrCustomerDeleteFail = NewAppError("CUSTOMER_DELETE_FAILED", "unable to delete customer", http.StatusInternalServerError)

	// Product errors
	ErrProductNotFound          = NewAppError("PRODUCT_NOT_FOUND", "product not found", http.StatusNotFound)
	ErrProductCreateFail        = NewAppError("PRODUCT_CREATION_FAILED", "unable to create product", http.StatusInternalServerError)
	ErrProductUpdateFail        = NewAppError("PRODUCT_UPDATE_FAILED", "unable to update product", http.StatusInternalServerError)
	ErrProductDeleteFail        = NewAppError("PRODUCT_DELETE_FAILED", "unable to delete product", http.StatusInternalServerError)
	ErrProductInsufficientStock = NewAppError("INSUFFICIENT_STOCK", "product stock is insufficient", http.StatusBadRequest)
	ErrProductInvalidPrice      = NewAppError("INVALID_PRICE", "product price must be greater than 0", http.StatusBadRequest)
	ErrProductInvalidStock      = NewAppError("INVALID_STOCK", "product stock cannot be negative", http.StatusBadRequest)
	ErrProductInvalidName       = NewAppError("INVALID_NAME", "product name is required", http.StatusBadRequest)
	ErrProductOutOfStock        = NewAppError("PRODUCT_OUT_OF_STOCK", "product is out of stock", http.StatusBadRequest)

	// Order errors
	ErrOrderNotFound          = NewAppError("ORDER_NOT_FOUND", "order not found", http.StatusNotFound)
	ErrOrderCreateFail        = NewAppError("ORDER_CREATION_FAILED", "unable to create order", http.StatusInternalServerError)
	ErrOrderUpdateFail        = NewAppError("ORDER_UPDATE_FAILED", "unable to update order", http.StatusInternalServerError)
	ErrOrderDeleteFail        = NewAppError("ORDER_DELETE_FAILED", "unable to delete order", http.StatusInternalServerError)
	ErrOrderInvalidStatus     = NewAppError("INVALID_ORDER_STATUS", "invalid order status", http.StatusBadRequest)
	ErrOrderEmptyItems        = NewAppError("EMPTY_ORDER_ITEMS", "order must contain at least one item", http.StatusBadRequest)
	ErrOrderInvalidCustomer   = NewAppError("INVALID_CUSTOMER", "customer does not exist", http.StatusBadRequest)
	ErrOrderInvalidProduct    = NewAppError("INVALID_PRODUCT", "one or more products do not exist", http.StatusBadRequest)
	ErrOrderInsufficientStock = NewAppError("ORDER_INSUFFICIENT_STOCK", "insufficient stock for one or more products", http.StatusBadRequest)
	ErrOrderTotalMismatch     = NewAppError("ORDER_TOTAL_MISMATCH", "order total calculation mismatch", http.StatusInternalServerError)
	ErrOrderAlreadyProcessed  = NewAppError("ORDER_ALREADY_PROCESSED", "order has already been processed and cannot be modified", http.StatusConflict)

	// Order Item errors
	ErrOrderItemInvalidQuantity = NewAppError("INVALID_QUANTITY", "item quantity must be greater than 0", http.StatusBadRequest)
	ErrOrderItemInvalidPrice    = NewAppError("INVALID_ITEM_PRICE", "item price must be greater than 0", http.StatusBadRequest)
	ErrOrderItemNotFound        = NewAppError("ORDER_ITEM_NOT_FOUND", "order item not found", http.StatusNotFound)

	// Transaction errors
	ErrTransactionBegin    = NewAppError("TRANSACTION_BEGIN_FAILED", "failed to begin transaction", http.StatusInternalServerError)
	ErrTransactionCommit   = NewAppError("TRANSACTION_COMMIT_FAILED", "failed to commit transaction", http.StatusInternalServerError)
	ErrTransactionRollback = NewAppError("TRANSACTION_ROLLBACK_FAILED", "failed to rollback transaction", http.StatusInternalServerError)

	// User errors
	ErrUserNotFound       = NewAppError("USER_NOT_FOUND", "user not found", http.StatusNotFound)
	ErrUserCreateFail     = NewAppError("USER_CREATION_FAILED", "unable to create user", http.StatusInternalServerError)
	ErrUserAlreadyExists  = NewAppError("USER_ALREADY_EXISTS", "email already registered", http.StatusBadRequest)
	ErrInvalidCredentials = NewAppError("INVALID_CREDENTIALS", "email or password incorrect", http.StatusUnauthorized)

	// Shop errors (multi-tenant)
	ErrShopNotFound    = NewAppError("SHOP_NOT_FOUND", "shop not found", http.StatusNotFound)
	ErrShopCreateFail  = NewAppError("SHOP_CREATION_FAILED", "unable to create shop", http.StatusInternalServerError)
	ErrShopUpdateFail  = NewAppError("SHOP_UPDATE_FAILED", "unable to update shop", http.StatusInternalServerError)
	ErrShopSlugTaken   = NewAppError("SHOP_SLUG_TAKEN", "this slug is already taken", http.StatusConflict)
	ErrShopDomainTaken = NewAppError("SHOP_DOMAIN_TAKEN", "this custom domain is already taken", http.StatusConflict)
	ErrShopNotOwner    = NewAppError("SHOP_NOT_OWNER", "you are not the owner of this shop", http.StatusForbidden)
	ErrShopInvalidID   = NewAppError("SHOP_INVALID_ID", "invalid shop ID", http.StatusBadRequest)
)
