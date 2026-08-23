package store

import (
	"database/sql"
	"time"

	"task165-collation/internal/model"
)

// CreateProject inserts a new project in draft status.
func (s *Store) CreateProject(p *model.Project) error {
	p.CreatedAt = time.Now().UTC()
	p.UpdatedAt = p.CreatedAt
	if p.Status == "" {
		p.Status = model.ProjectDraft
	}
	if p.Version == 0 {
		p.Version = 1
	}
	_, err := s.db.Exec(`INSERT INTO projects (id, name, description, status, base_witness_id, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, string(p.Status), p.BaseWitnessID, p.Version, p.CreatedAt, p.UpdatedAt)
	return err
}

// GetProject loads a project by id.
func (s *Store) GetProject(id string) (*model.Project, error) {
	row := s.db.QueryRow(`SELECT id, name, description, status, base_witness_id, version, created_at, updated_at FROM projects WHERE id = ?`, id)
	return scanProject(row)
}

// ListProjects returns all projects ordered by creation time.
func (s *Store) ListProjects() ([]*model.Project, error) {
	rows, err := s.db.Query(`SELECT id, name, description, status, base_witness_id, version, created_at, updated_at FROM projects ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpdateProjectStatus applies a status change with optimistic locking on the
// version column. Returns model.ErrConflict when the version does not match.
func (s *Store) UpdateProjectStatus(id string, expectVersion int64, status model.ProjectStatus) error {
	res, err := s.db.Exec(`UPDATE projects SET status = ?, version = version + 1, updated_at = ? WHERE id = ? AND version = ?`,
		string(status), now(), id, expectVersion)
	if err != nil {
		return err
	}
	return checkRows(res, "project status")
}

// SetProjectBaseWitness records the base witness of a project.
func (s *Store) SetProjectBaseWitness(id, baseWitnessID string) error {
	res, err := s.db.Exec(`UPDATE projects SET base_witness_id = ?, updated_at = ? WHERE id = ?`, baseWitnessID, now(), id)
	if err != nil {
		return err
	}
	return checkRows(res, "project base")
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProject(r rowScanner) (*model.Project, error) {
	var p model.Project
	var created, updated string
	if err := r.Scan(&p.ID, &p.Name, &p.Description, (*string)(&p.Status), &p.BaseWitnessID, &p.Version, &created, &updated); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	p.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return &p, nil
}
