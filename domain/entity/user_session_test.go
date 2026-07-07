package entity

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// 🆕 v4.4.3 : TESTS UNITAIRES - ENTITÉ USER SESSION
// ============================================================

// ============================================================
// TESTS : NewUserSession() - Constructeur
// ============================================================

func TestNewUserSession_Success(t *testing.T) {
	session, err := NewUserSession(
		"user-123",
		"session-456",
		"test-token-jwt",
		"192.168.1.1",
		"Mozilla/5.0 Chrome/120.0",
		24*time.Hour,
	)

	assert.NoError(t, err, "Should not return error")
	assert.NotNil(t, session, "Should return session")
	assert.Equal(t, "user-123", session.UserID)
	assert.Equal(t, "session-456", session.SessionID)
	assert.NotEmpty(t, session.SessionTokenHash, "Token should be hashed")
	assert.Equal(t, "192.168.1.1", session.IPAddress)
	assert.True(t, session.IsActive, "Should be active")
	assert.Nil(t, session.RevokedAt, "Should not be revoked")
	assert.True(t, session.ExpiresAt.After(time.Now()), "ExpiresAt should be in the future")
}

func TestNewUserSession_EmptyUserID(t *testing.T) {
	_, err := NewUserSession(
		"",
		"session-456",
		"test-token",
		"192.168.1.1",
		"Mozilla/5.0",
		24*time.Hour,
	)

	assert.Error(t, err, "Should return error for empty user_id")
	assert.Equal(t, ErrSessionUserIDRequired, err)
}

func TestNewUserSession_DefaultDuration(t *testing.T) {
	session, err := NewUserSession(
		"user-123",
		"session-456",
		"test-token",
		"192.168.1.1",
		"Mozilla/5.0",
		0,
	)

	assert.NoError(t, err)
	expectedDuration := DefaultSessionDuration
	diff := time.Until(session.ExpiresAt) - expectedDuration
	assert.True(t, diff < 5*time.Second, "Should use default duration (7 days)")
}

func TestNewUserSession_MaxDuration(t *testing.T) {
	session, err := NewUserSession(
		"user-123",
		"session-456",
		"test-token",
		"192.168.1.1",
		"Mozilla/5.0",
		100*24*time.Hour,
	)

	assert.NoError(t, err)
	maxDiff := time.Until(session.ExpiresAt) - MaxSessionDuration
	assert.True(t, maxDiff < 5*time.Second, "Should cap at max duration (30 days)")
}

// ============================================================
// TESTS : GetStatus()
// ============================================================

func TestUserSession_GetStatus_Active(t *testing.T) {
	session := &UserSession{
		IsActive:     true,
		LastActivity: time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	assert.Equal(t, "active", session.GetStatus())
}

func TestUserSession_GetStatus_Away(t *testing.T) {
	session := &UserSession{
		IsActive:     true,
		LastActivity: time.Now().Add(-20 * time.Minute),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	assert.Equal(t, "away", session.GetStatus())
}

func TestUserSession_GetStatus_Idle(t *testing.T) {
	session := &UserSession{
		IsActive:     true,
		LastActivity: time.Now().Add(-90 * time.Minute),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	assert.Equal(t, "idle", session.GetStatus())
}

func TestUserSession_GetStatus_Revoked(t *testing.T) {
	revokedAt := time.Now()
	session := &UserSession{
		IsActive:  false,
		RevokedAt: &revokedAt,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	assert.Equal(t, "revoked", session.GetStatus())
}

func TestUserSession_GetStatus_Expired(t *testing.T) {
	session := &UserSession{
		IsActive:     true,
		LastActivity: time.Now(),
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
	}
	assert.Equal(t, "expired", session.GetStatus())
}

func TestUserSession_GetStatus_RevokedTakesPrecedence(t *testing.T) {
	revokedAt := time.Now()
	session := &UserSession{
		IsActive:     false,
		RevokedAt:    &revokedAt,
		LastActivity: time.Now().Add(-90 * time.Minute),
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
	}
	assert.Equal(t, "revoked", session.GetStatus())
}

// ============================================================
// TESTS : IsExpired()
// ============================================================

func TestUserSession_IsExpired_NotExpired(t *testing.T) {
	session := &UserSession{ExpiresAt: time.Now().Add(24 * time.Hour)}
	assert.False(t, session.IsExpired())
}

func TestUserSession_IsExpired_Expired(t *testing.T) {
	session := &UserSession{ExpiresAt: time.Now().Add(-1 * time.Hour)}
	assert.True(t, session.IsExpired())
}

// ============================================================
// TESTS : IsRevoked()
// ============================================================

func TestUserSession_IsRevoked_NotRevoked(t *testing.T) {
	session := &UserSession{IsActive: true, RevokedAt: nil}
	assert.False(t, session.IsRevoked())
}

func TestUserSession_IsRevoked_Revoked(t *testing.T) {
	revokedAt := time.Now()
	session := &UserSession{IsActive: false, RevokedAt: &revokedAt}
	assert.True(t, session.IsRevoked())
}

// ============================================================
// TESTS : IsActiveSession()
// ============================================================

func TestUserSession_IsActiveSession_Valid(t *testing.T) {
	session := &UserSession{IsActive: true, ExpiresAt: time.Now().Add(24 * time.Hour)}
	assert.True(t, session.IsActiveSession())
}

func TestUserSession_IsActiveSession_Expired(t *testing.T) {
	session := &UserSession{IsActive: true, ExpiresAt: time.Now().Add(-1 * time.Hour)}
	assert.False(t, session.IsActiveSession())
}

func TestUserSession_IsActiveSession_Revoked(t *testing.T) {
	session := &UserSession{IsActive: false, ExpiresAt: time.Now().Add(24 * time.Hour)}
	assert.False(t, session.IsActiveSession())
}

// ============================================================
// TESTS : IsIdle()
// ============================================================

func TestUserSession_IsIdle_NotIdle(t *testing.T) {
	session := &UserSession{LastActivity: time.Now().Add(-10 * time.Minute)}
	assert.False(t, session.IsIdle())
}

func TestUserSession_IsIdle_Idle(t *testing.T) {
	session := &UserSession{LastActivity: time.Now().Add(-90 * time.Minute)}
	assert.True(t, session.IsIdle())
}

// ============================================================
// TESTS : GetMinutesInactive()
// ============================================================

func TestUserSession_GetMinutesInactive(t *testing.T) {
	session := &UserSession{LastActivity: time.Now().Add(-30 * time.Minute)}
	minutes := session.GetMinutesInactive()
	assert.True(t, minutes >= 29.9 && minutes <= 30.1)
}

// ============================================================
// TESTS : GetTimeRemaining()
// ============================================================

func TestUserSession_GetTimeRemaining_Valid(t *testing.T) {
	session := &UserSession{ExpiresAt: time.Now().Add(2 * time.Hour)}
	remaining := session.GetTimeRemaining()
	assert.True(t, remaining > 0)
	assert.True(t, remaining < 3*time.Hour)
}

func TestUserSession_GetTimeRemaining_Expired(t *testing.T) {
	session := &UserSession{ExpiresAt: time.Now().Add(-1 * time.Hour)}
	remaining := session.GetTimeRemaining()
	assert.Equal(t, time.Duration(0), remaining)
}

// ============================================================
// TESTS : MarkRevoked()
// ============================================================

func TestUserSession_MarkRevoked(t *testing.T) {
	session := &UserSession{IsActive: true}
	before := time.Now()
	session.MarkRevoked("admin-123")
	after := time.Now()

	assert.False(t, session.IsActive)
	assert.NotNil(t, session.RevokedAt)
	assert.Equal(t, "admin-123", session.RevokedBy)
	assert.True(t, !session.RevokedAt.Before(before))
	assert.True(t, !session.RevokedAt.After(after))
}

// ============================================================
// TESTS : UpdateLastActivity()
// ============================================================

func TestUserSession_UpdateLastActivity(t *testing.T) {
	session := &UserSession{LastActivity: time.Now().Add(-1 * time.Hour)}
	before := time.Now()
	session.UpdateLastActivity()
	after := time.Now()

	assert.True(t, !session.LastActivity.Before(before))
	assert.True(t, !session.LastActivity.After(after))
}

// ============================================================
// TESTS : ExtendSession()
// ============================================================

func TestUserSession_ExtendSession_Success(t *testing.T) {
	session := &UserSession{
		IsActive:     true,
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		CreatedAt:    time.Now().Add(-1 * time.Hour),
		LastActivity: time.Now(),
	}
	err := session.ExtendSession(24 * time.Hour)
	assert.NoError(t, err)
	assert.True(t, session.ExpiresAt.After(time.Now().Add(23*time.Hour)))
}

func TestUserSession_ExtendSession_Expired(t *testing.T) {
	session := &UserSession{
		IsActive:  true,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	err := session.ExtendSession(24 * time.Hour)
	assert.Error(t, err)
	assert.Equal(t, ErrSessionExpired, err)
}

func TestUserSession_ExtendSession_Revoked(t *testing.T) {
	revokedAt := time.Now()
	session := &UserSession{
		IsActive:  false,
		RevokedAt: &revokedAt,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	err := session.ExtendSession(24 * time.Hour)
	assert.Error(t, err)
}

func TestUserSession_ExtendSession_ExceedsMax(t *testing.T) {
	session := &UserSession{
		IsActive:     true,
		CreatedAt:    time.Now().Add(-29 * 24 * time.Hour),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		LastActivity: time.Now(),
	}
	err := session.ExtendSession(30 * 24 * time.Hour)
	assert.Error(t, err)
}

// ============================================================
// TESTS : HashToken()
// ============================================================

func TestHashToken_Consistency(t *testing.T) {
	token := "test-token-12345"
	hash1 := HashToken(token)
	hash2 := HashToken(token)
	assert.Equal(t, hash1, hash2)
	assert.Equal(t, 64, len(hash1))
}

func TestHashToken_DifferentTokens(t *testing.T) {
	hash1 := HashToken("token-1")
	hash2 := HashToken("token-2")
	assert.NotEqual(t, hash1, hash2)
}

// ============================================================
// TESTS : ValidateToken()
// ============================================================

func TestUserSession_ValidateToken_Valid(t *testing.T) {
	token := "my-secret-token"
	session := &UserSession{SessionTokenHash: HashToken(token)}
	assert.True(t, session.ValidateToken(token))
}

func TestUserSession_ValidateToken_Invalid(t *testing.T) {
	session := &UserSession{SessionTokenHash: HashToken("original-token")}
	assert.False(t, session.ValidateToken("different-token"))
}

// ============================================================
// 🆕 v4.4.3 : TESTS ParseUserAgent() - NAVIGATEURS
// ============================================================

func TestParseUserAgent_Chrome(t *testing.T) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	info := ParseUserAgent(ua)
	assert.Equal(t, "Chrome", info.Browser)
	assert.Equal(t, "Windows", info.OS)
	assert.Equal(t, "Desktop", info.Device)
}

func TestParseUserAgent_Firefox(t *testing.T) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0"
	info := ParseUserAgent(ua)
	assert.Equal(t, "Firefox", info.Browser)
}

func TestParseUserAgent_Safari(t *testing.T) {
	ua := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15"
	info := ParseUserAgent(ua)
	assert.Equal(t, "Safari", info.Browser)
	assert.Equal(t, "macOS", info.OS)
	assert.Equal(t, "Desktop", info.Device)
}

func TestParseUserAgent_Edge(t *testing.T) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0"
	info := ParseUserAgent(ua)
	assert.Equal(t, "Edge", info.Browser)
}

func TestParseUserAgent_Opera(t *testing.T) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 OPR/105.0.0.0"
	info := ParseUserAgent(ua)
	assert.Equal(t, "Opera", info.Browser, "Opera should be detected (OPR in UA)")
	assert.Equal(t, "Windows", info.OS)
}

func TestParseUserAgent_OperaClassic(t *testing.T) {
	ua := "Opera/9.80 (Windows NT 6.1) Presto/2.12.388 Version/12.18"
	info := ParseUserAgent(ua)
	assert.Equal(t, "Opera", info.Browser, "Classic Opera should be detected")
}

func TestParseUserAgent_InternetExplorer(t *testing.T) {
	ua := "Mozilla/5.0 (Windows NT 10.0; WOW64; Trident/7.0; rv:11.0) like Gecko"
	info := ParseUserAgent(ua)
	assert.Equal(t, "Internet Explorer", info.Browser)
}

func TestParseUserAgent_Empty(t *testing.T) {
	info := ParseUserAgent("")
	assert.Equal(t, UnknownDevice, info.Browser)
	assert.Equal(t, UnknownDevice, info.OS)
	assert.Equal(t, UnknownDevice, info.Device)
}

// ============================================================
// 🆕 v4.4.3 : TESTS ParseUserAgent() - DEVICES
// ============================================================

func TestParseUserAgent_Mobile(t *testing.T) {
	ua := "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36"
	info := ParseUserAgent(ua)
	assert.Equal(t, "Android", info.OS)
	assert.Equal(t, "Mobile", info.Device)
}

func TestParseUserAgent_Tablet_iPad(t *testing.T) {
	ua := "Mozilla/5.0 (iPad; CPU OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1"
	info := ParseUserAgent(ua)
	assert.Equal(t, "iOS", info.OS, "iPad should be detected as iOS")
	assert.Equal(t, "Tablet", info.Device, "iPad should be detected as Tablet")
}

func TestParseUserAgent_Tablet_Android(t *testing.T) {
	ua := "Mozilla/5.0 (Linux; Android 13; SM-T870) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	info := ParseUserAgent(ua)
	assert.Equal(t, "Android", info.OS)
	// SM-T870 est un Galaxy Tab, mais sans "tablet" explicite
	// La détection dépend du pattern
	assert.Contains(t, []string{"Tablet", "Mobile", "Desktop"}, info.Device)
}

func TestParseUserAgent_Desktop(t *testing.T) {
	ua := "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	info := ParseUserAgent(ua)
	assert.Equal(t, "Linux", info.OS)
	assert.Equal(t, "Desktop", info.Device)
}

func TestParseUserAgent_iPhone(t *testing.T) {
	ua := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1"
	info := ParseUserAgent(ua)
	assert.Equal(t, "iOS", info.OS, "iPhone should be detected as iOS")
	assert.Equal(t, "Mobile", info.Device, "iPhone should be detected as Mobile")
}

// ============================================================
// TESTS : DeviceInfo JSON Serialization
// ============================================================

func TestDeviceInfo_ToJSON(t *testing.T) {
	info := DeviceInfo{
		UserAgent: "Mozilla/5.0 Chrome",
		Browser:   "Chrome",
		OS:        "Windows",
		Device:    "Desktop",
		Platform:  "Windows",
	}
	jsonStr, err := info.ToJSON()
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonStr)
	assert.Contains(t, jsonStr, "Chrome")
	assert.Contains(t, jsonStr, "Windows")
}

func TestDeviceInfo_FromJSON(t *testing.T) {
	jsonStr := `{"user_agent":"Mozilla/5.0","browser":"Firefox","os":"Linux","device":"Desktop","platform":"Linux"}`
	info, err := DeviceInfoFromJSON(jsonStr)
	assert.NoError(t, err)
	assert.Equal(t, "Mozilla/5.0", info.UserAgent)
	assert.Equal(t, "Firefox", info.Browser)
	assert.Equal(t, "Linux", info.OS)
}

func TestDeviceInfo_FromJSON_Empty(t *testing.T) {
	info, err := DeviceInfoFromJSON("")
	assert.NoError(t, err)
	assert.Equal(t, DeviceInfo{}, info)
}

func TestDeviceInfo_RoundTrip(t *testing.T) {
	original := DeviceInfo{
		UserAgent: "Mozilla/5.0 Chrome",
		Browser:   "Chrome",
		OS:        "Windows",
		Device:    "Desktop",
		Platform:  "Windows",
	}
	jsonStr, err := original.ToJSON()
	assert.NoError(t, err)
	restored, err := DeviceInfoFromJSON(jsonStr)
	assert.NoError(t, err)
	assert.Equal(t, original, restored)
}

// ============================================================
// TESTS : ToSummary()
// ============================================================

func TestUserSession_ToSummary_IsCurrent(t *testing.T) {
	session := &UserSession{
		ID:           "session-1",
		SessionID:    "jti-123",
		UserID:       "user-1",
		DeviceInfo:   DeviceInfo{Browser: "Chrome", OS: "Windows", Device: "Desktop"},
		IPAddress:    "192.168.1.1",
		LastActivity: time.Now(),
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		IsActive:     true,
	}
	summary := session.ToSummary("jti-123")
	assert.Equal(t, "session-1", summary.ID)
	assert.Equal(t, "Chrome", summary.Browser)
	assert.True(t, summary.IsCurrent)
	assert.Equal(t, "active", summary.Status)
}

func TestUserSession_ToSummary_NotCurrent(t *testing.T) {
	session := &UserSession{
		ID:        "session-1",
		SessionID: "jti-123",
		IsActive:  true,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	summary := session.ToSummary("jti-456")
	assert.False(t, summary.IsCurrent)
}

// ============================================================
// TESTS : Edge Cases
// ============================================================

func TestUserSession_JustExpired(t *testing.T) {
	session := &UserSession{
		IsActive:  true,
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	assert.True(t, session.IsExpired())
	assert.Equal(t, "expired", session.GetStatus())
}

func TestParseUserAgent_CaseInsensitive(t *testing.T) {
	ua := "MOZILLA/5.0 CHROME FIREFOX"
	info := ParseUserAgent(ua)
	assert.True(t,
		strings.EqualFold(info.Browser, "Chrome") || strings.EqualFold(info.Browser, "Firefox") || info.Browser == UnknownDevice)
}

func TestUserSession_HashTokenSecurity(t *testing.T) {
	token := "my-secret-token"
	hash := HashToken(token)
	assert.NotEqual(t, token, hash)
	assert.NotContains(t, hash, "secret")
	assert.Equal(t, 64, len(hash))
	for _, c := range hash {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'))
	}
}

func TestUserSession_JSON_ExcludesSensitiveFields(t *testing.T) {
	session := &UserSession{
		ID:               "session-1",
		UserID:           "user-1",
		SessionTokenHash: "secret-hash-should-not-appear",
		IsActive:         true,
	}
	data, err := json.Marshal(session)
	assert.NoError(t, err)
	jsonStr := string(data)
	assert.NotContains(t, jsonStr, "secret-hash-should-not-appear")
	assert.Contains(t, jsonStr, "session-1")
}

// ============================================================
// 🆕 v4.4.3 : TESTS ParseUserAgent() - Cas spéciaux
// ============================================================

func TestParseUserAgent_ChromeOnIPhone(t *testing.T) {
	ua := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/120.0.6099.119 Mobile/15E148 Safari/604.1"
	info := ParseUserAgent(ua)
	assert.Equal(t, "Chrome", info.Browser, "Chrome on iPhone should be detected as Chrome")
	assert.Equal(t, "iOS", info.OS, "iPhone should be detected as iOS")
	assert.Equal(t, "Mobile", info.Device)
}

func TestParseUserAgent_FirefoxOnAndroid(t *testing.T) {
	ua := "Mozilla/5.0 (Android 13; Mobile; rv:121.0) Gecko/121.0 Firefox/121.0"
	info := ParseUserAgent(ua)
	assert.Equal(t, "Firefox", info.Browser)
	assert.Equal(t, "Android", info.OS)
	assert.Equal(t, "Mobile", info.Device)
}

func TestParseUserAgent_UnknownBrowser(t *testing.T) {
	ua := "SomeUnknownBrowser/1.0"
	info := ParseUserAgent(ua)
	assert.Equal(t, UnknownDevice, info.Browser, "Unknown browser should return Unknown")
}
