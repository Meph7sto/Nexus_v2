package middleware

import "github.com/gin-gonic/gin"

type PrincipalKind string

const (
	PrincipalKindHuman       PrincipalKind = "human"
	PrincipalKindAdminAPIKey PrincipalKind = "admin_api_key"
)

// AuthSubject is the minimal authenticated identity stored in gin context.
// Decision: {UserID int64, Concurrency int}
type AuthSubject struct {
	UserID        int64
	Concurrency   int
	PrincipalKind PrincipalKind
}

func (s AuthSubject) IsHuman() bool {
	// Empty preserves compatibility with context values produced before the
	// principal-kind field was introduced; all newly authenticated JWT users
	// explicitly carry PrincipalKindHuman.
	return s.PrincipalKind == "" || s.PrincipalKind == PrincipalKindHuman
}

func GetAuthSubjectFromContext(c *gin.Context) (AuthSubject, bool) {
	value, exists := c.Get(string(ContextKeyUser))
	if !exists {
		return AuthSubject{}, false
	}
	subject, ok := value.(AuthSubject)
	return subject, ok
}

func GetUserRoleFromContext(c *gin.Context) (string, bool) {
	value, exists := c.Get(string(ContextKeyUserRole))
	if !exists {
		return "", false
	}
	role, ok := value.(string)
	return role, ok
}
