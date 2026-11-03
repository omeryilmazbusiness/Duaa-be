package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/domain/work"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errx.NotFound("not_found", "not found")
	}
	return err
}

func (s *Store) CreateOrganization(ctx context.Context, o tenancy.Organization) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO organizations (id, name, created_at, updated_at) VALUES ($1,$2,$3,$4)`,
		o.ID, o.Name, o.CreatedAt, o.UpdatedAt)
	return err
}

func (s *Store) CreateWorkspace(ctx context.Context, w tenancy.Workspace) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO workspaces (id, organization_id, kind, name, slug, timezone, is_active, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		w.ID, w.OrganizationID, w.Kind, w.Name, w.Slug, w.Timezone, w.IsActive, w.CreatedAt, w.UpdatedAt)
	if isUnique(err) {
		return errx.Conflict("slug_taken", "workspace slug already exists")
	}
	return err
}

func (s *Store) GetWorkspace(ctx context.Context, wid id.ID) (tenancy.Workspace, error) {
	var w tenancy.Workspace
	err := s.pool.QueryRow(ctx, `SELECT id, organization_id, kind, name, slug, timezone, is_active, created_at, updated_at FROM workspaces WHERE id=$1`, wid).
		Scan(&w.ID, &w.OrganizationID, &w.Kind, &w.Name, &w.Slug, &w.Timezone, &w.IsActive, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return tenancy.Workspace{}, mapErr(err)
	}
	return w, nil
}

func (s *Store) GetWorkspaceBySlug(ctx context.Context, slug string) (tenancy.Workspace, error) {
	var w tenancy.Workspace
	err := s.pool.QueryRow(ctx, `SELECT id, organization_id, kind, name, slug, timezone, is_active, created_at, updated_at FROM workspaces WHERE slug=$1`, slug).
		Scan(&w.ID, &w.OrganizationID, &w.Kind, &w.Name, &w.Slug, &w.Timezone, &w.IsActive, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return tenancy.Workspace{}, mapErr(err)
	}
	return w, nil
}

func (s *Store) CreateUser(ctx context.Context, u identity.User) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, first_name, last_name, is_active, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		u.ID, nullIfEmpty(u.Email), nullIfEmpty(u.PasswordHash), u.FirstName, u.LastName, u.IsActive, u.CreatedAt, u.UpdatedAt)
	if isUnique(err) {
		return errx.Conflict("email_taken", "a user with this email address already exists")
	}
	return err
}

func (s *Store) GetUser(ctx context.Context, uid id.ID) (identity.User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `SELECT id, coalesce(email,''), coalesce(password_hash,''), first_name, last_name, is_active, created_at, updated_at FROM users WHERE id=$1`, uid))
	if err != nil {
		return identity.User{}, mapErr(err)
	}
	return u, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (identity.User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `SELECT id, coalesce(email,''), coalesce(password_hash,''), first_name, last_name, is_active, created_at, updated_at FROM users WHERE email=$1`, email))
	if err != nil {
		return identity.User{}, mapErr(err)
	}
	return u, nil
}

func (s *Store) UpdateUser(ctx context.Context, u identity.User) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET email=$2, password_hash=$3, first_name=$4, last_name=$5, is_active=$6, updated_at=$7 WHERE id=$1`,
		u.ID, nullIfEmpty(u.Email), nullIfEmpty(u.PasswordHash), u.FirstName, u.LastName, u.IsActive, u.UpdatedAt)
	return err
}

func scanUser(row pgx.Row) (identity.User, error) {
	var u identity.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (s *Store) CreateMembership(ctx context.Context, m identity.Membership) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO memberships (id, workspace_id, user_id, role, department, employee_number, is_active, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		m.ID, m.WorkspaceID, m.UserID, m.Role, nullIfEmpty(m.Department), nullIfEmpty(m.EmployeeNumber), m.IsActive, m.CreatedAt, m.UpdatedAt)
	if isUnique(err) {
		return errx.Conflict("membership_exists", "membership already exists")
	}
	return err
}

func (s *Store) GetMembership(ctx context.Context, mid id.ID) (identity.Membership, error) {
	m, err := scanMembership(s.pool.QueryRow(ctx, membershipSelect+` WHERE id=$1`, mid))
	if err != nil {
		return identity.Membership{}, mapErr(err)
	}
	return m, nil
}

const membershipSelect = `SELECT id, workspace_id, user_id, role, coalesce(department,''), coalesce(employee_number,''), is_active, created_at, updated_at FROM memberships`

func scanMembership(row pgx.Row) (identity.Membership, error) {
	var m identity.Membership
	err := row.Scan(&m.ID, &m.WorkspaceID, &m.UserID, &m.Role, &m.Department, &m.EmployeeNumber, &m.IsActive, &m.CreatedAt, &m.UpdatedAt)
	return m, err
}

func (s *Store) ListMembershipsByUser(ctx context.Context, userID id.ID) ([]identity.Membership, error) {
	return s.queryMemberships(ctx, membershipSelect+` WHERE user_id=$1`, userID)
}

func (s *Store) ListWorkspaceMemberships(ctx context.Context, workspaceID id.ID) ([]identity.Membership, error) {
	return s.queryMemberships(ctx, membershipSelect+` WHERE workspace_id=$1`, workspaceID)
}

func (s *Store) queryMemberships(ctx context.Context, q string, arg any) ([]identity.Membership, error) {
	rows, err := s.pool.Query(ctx, q, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]identity.Membership, 0)
	for rows.Next() {
		m, err := scanMembership(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) UpdateMembership(ctx context.Context, m identity.Membership) error {
	_, err := s.pool.Exec(ctx, `UPDATE memberships SET role=$2, department=$3, employee_number=$4, is_active=$5, updated_at=$6 WHERE id=$1`,
		m.ID, m.Role, nullIfEmpty(m.Department), nullIfEmpty(m.EmployeeNumber), m.IsActive, m.UpdatedAt)
	return err
}

func (s *Store) GetMembershipByEmployee(ctx context.Context, workspaceID id.ID, employeeNumber string) (identity.Membership, error) {
	m, err := scanMembership(s.pool.QueryRow(ctx, membershipSelect+` WHERE workspace_id=$1 AND employee_number=$2`, workspaceID, employeeNumber))
	if err != nil {
		return identity.Membership{}, mapErr(err)
	}
	return m, nil
}

func (s *Store) GetActiveDepartmentManager(ctx context.Context, workspaceID id.ID, department string) (identity.Membership, error) {
	m, err := scanMembership(s.pool.QueryRow(ctx, membershipSelect+` WHERE workspace_id=$1 AND role='manager' AND department=$2 AND is_active=true`, workspaceID, department))
	if err != nil {
		return identity.Membership{}, mapErr(err)
	}
	return m, nil
}

func (s *Store) CreateTask(ctx context.Context, t work.Task) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO staff_tasks (id, workspace_id, assignee_user_id, created_by_user_id, title, description, scheduled_start_at, scheduled_end_at, status, routine_id, source_date, completed_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		t.ID, t.WorkspaceID, t.AssigneeUserID, t.CreatedByUserID, t.Title, nullIfEmpty(t.Description), t.ScheduledStart, t.ScheduledEnd, t.Status, uuidOrNil(t.RoutineID), nullIfEmpty(t.SourceDate), t.CompletedAt, t.CreatedAt, t.UpdatedAt)
	return err
}

func (s *Store) GetTask(ctx context.Context, tid id.ID) (work.Task, error) {
	t, err := scanTask(s.pool.QueryRow(ctx, taskSelect+` WHERE id=$1`, tid))
	if err != nil {
		return work.Task{}, mapErr(err)
	}
	return t, nil
}

// source_date is DATE — must cast before coalesce('', …) or Postgres rejects the query.
const taskSelect = `SELECT id, workspace_id, assignee_user_id, created_by_user_id, title, coalesce(description,''), scheduled_start_at, scheduled_end_at, status, coalesce(routine_id,'00000000-0000-0000-0000-000000000000'), coalesce(source_date::text,''), completed_at, created_at, updated_at FROM staff_tasks`

func scanTask(row pgx.Row) (work.Task, error) {
	var t work.Task
	var completed *time.Time
	err := row.Scan(&t.ID, &t.WorkspaceID, &t.AssigneeUserID, &t.CreatedByUserID, &t.Title, &t.Description, &t.ScheduledStart, &t.ScheduledEnd, &t.Status, &t.RoutineID, &t.SourceDate, &completed, &t.CreatedAt, &t.UpdatedAt)
	t.CompletedAt = completed
	return t, err
}

func (s *Store) ListTasksByWorkspace(ctx context.Context, workspaceID id.ID) ([]work.Task, error) {
	return s.queryTasks(ctx, taskSelect+` WHERE workspace_id=$1`, workspaceID)
}

func (s *Store) ListTasksByAssignee(ctx context.Context, workspaceID, assignee id.ID) ([]work.Task, error) {
	rows, err := s.pool.Query(ctx, taskSelect+` WHERE workspace_id=$1 AND assignee_user_id=$2`, workspaceID, assignee)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTasks(rows)
}

func (s *Store) queryTasks(ctx context.Context, q string, arg any) ([]work.Task, error) {
	rows, err := s.pool.Query(ctx, q, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTasks(rows)
}

func collectTasks(rows pgx.Rows) ([]work.Task, error) {
	out := make([]work.Task, 0)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) UpdateTask(ctx context.Context, t work.Task) error {
	_, err := s.pool.Exec(ctx, `UPDATE staff_tasks SET status=$2, completed_at=$3, updated_at=$4 WHERE id=$1 AND workspace_id=$5`,
		t.ID, t.Status, t.CompletedAt, t.UpdatedAt, t.WorkspaceID)
	return err
}

func (s *Store) CreateRoutine(ctx context.Context, r work.Routine) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO routine_tasks (id, workspace_id, created_by_user_id, assignee_user_id, title, description, start_time, end_time, is_active, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		r.ID, r.WorkspaceID, r.CreatedByUserID, r.AssigneeUserID, r.Title, nullIfEmpty(r.Description), r.StartTime, r.EndTime, r.IsActive, r.CreatedAt, r.UpdatedAt)
	return err
}

func (s *Store) ListRoutinesByWorkspace(ctx context.Context, workspaceID id.ID) ([]work.Routine, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, workspace_id, created_by_user_id, assignee_user_id, title, coalesce(description,''), start_time, end_time, is_active, created_at, updated_at FROM routine_tasks WHERE workspace_id=$1`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]work.Routine, 0)
	for rows.Next() {
		var r work.Routine
		if err := rows.Scan(&r.ID, &r.WorkspaceID, &r.CreatedByUserID, &r.AssigneeUserID, &r.Title, &r.Description, &r.StartTime, &r.EndTime, &r.IsActive, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func uuidOrNil(v id.ID) any {
	if v == id.Zero() {
		return nil
	}
	return v
}

func isUnique(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}
