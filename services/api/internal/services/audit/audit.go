// Package audit provides a simple helper for recording events to the audit_log table.
// Used by handlers and background workers to track account-scoped actions.
package audit

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Log records an audit entry. accountID may be uuid.Nil for system-wide events.
// userID may be uuid.Nil for automated/system actions.
func Log(
	ctx context.Context,
	db *pgxpool.Pool,
	accountID uuid.UUID,
	userID uuid.UUID,
	action string,
	entityType string,
	entityID string,
	details map[string]interface{},
	ipAddress string,
) {
	detailsJSON, _ := json.Marshal(details)

	var accountArg interface{}
	if accountID != uuid.Nil {
		accountArg = accountID
	}
	var userArg interface{}
	if userID != uuid.Nil {
		userArg = userID
	}
	var entityArg interface{}
	if entityID != "" {
		if parsed, err := uuid.Parse(entityID); err == nil {
			entityArg = parsed
		}
	}
	var ipArg interface{}
	if ipAddress != "" {
		ipArg = ipAddress
	}

	_, _ = db.Exec(ctx, `
		INSERT INTO audit_log (id, account_id, user_id, action, entity_type, entity_id, details, ip_address, created_at)
		VALUES (uuid_generate_v4(), $1, $2, $3, NULLIF($4, ''), $5, $6, $7::inet, NOW())
	`, accountArg, userArg, action, entityType, entityArg, detailsJSON, ipArg)
}
