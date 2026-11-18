package postgres

import (
	"context"
	"encoding/json"

	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func (s *Store) SaveMeeting(ctx context.Context, rec assistant.MeetingRecord) error {
	partRaw, _ := json.Marshal(rec.Participants)
	decRaw, _ := json.Marshal(rec.Decisions)
	actRaw, _ := json.Marshal(rec.ActionItems)
	_, err := s.pool.Exec(ctx, `INSERT INTO assistant_meetings (
		id, workspace_id, created_by, business_date, title, participants, transcript,
		translated_transcript, summary, decisions, action_items, source_locale, target_locale, ai_used, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		rec.ID, rec.WorkspaceID, rec.CreatedBy, rec.BusinessDate, rec.Title, partRaw, rec.Transcript,
		rec.TranslatedTranscript, rec.Summary, decRaw, actRaw, rec.SourceLocale, rec.TargetLocale, rec.AIUsed, rec.CreatedAt)
	return err
}

func (s *Store) ListMeetings(ctx context.Context, workspaceID id.ID, limit int) ([]assistant.MeetingRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `SELECT id, workspace_id, created_by, business_date::text, title,
		participants, transcript, translated_transcript, summary, decisions, action_items,
		source_locale, target_locale, ai_used, created_at
		FROM assistant_meetings WHERE workspace_id=$1 ORDER BY created_at DESC LIMIT $2`, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]assistant.MeetingRecord, 0)
	for rows.Next() {
		var rec assistant.MeetingRecord
		var partRaw, decRaw, actRaw []byte
		if err := rows.Scan(
			&rec.ID, &rec.WorkspaceID, &rec.CreatedBy, &rec.BusinessDate, &rec.Title,
			&partRaw, &rec.Transcript, &rec.TranslatedTranscript, &rec.Summary, &decRaw, &actRaw,
			&rec.SourceLocale, &rec.TargetLocale, &rec.AIUsed, &rec.CreatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(partRaw, &rec.Participants)
		_ = json.Unmarshal(decRaw, &rec.Decisions)
		_ = json.Unmarshal(actRaw, &rec.ActionItems)
		out = append(out, rec)
	}
	return out, rows.Err()
}
