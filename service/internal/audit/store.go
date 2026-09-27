package audit

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"shamal/service/internal/platform"
)

type Event struct {
	ActorID    string
	Action     string
	EntityType string
	EntityID   string
	CompanyID  string
	RequestID  string
	Result     string
	Reason     string
	Before     any
	After      any
}

func Record(ctx context.Context, db platform.DB, event Event) error {
	before, _ := json.Marshal(event.Before)
	after, _ := json.Marshal(event.After)
	if event.Before == nil {
		before = nil
	}
	if event.After == nil {
		after = nil
	}
	_, err := db.Exec(ctx, `
		INSERT INTO audit.events (id, actor_id, action, entity_type, entity_id, company_id, request_id, result, reason, before_data, after_data)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, NULLIF($6,'')::uuid, NULLIF($7,''), $8, $9, $10, $11)
	`, uuid.NewString(), event.ActorID, event.Action, event.EntityType, event.EntityID, event.CompanyID, event.RequestID, event.Result, event.Reason, before, after)
	return err
}
