package pg

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type incidentRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r incidentRepository) Insert(entity *internal.IncidentEntity) error {
	row := r.tx.QueryRow(r.txCtx, `
INSERT INTO incident (
	partition,
	id,

	element_id,
	element_instance_id,
	job_id,
	process_id,
	process_instance_id,
	task_id,

	created_at,
	created_by
) VALUES (
	$1,
	nextval($2),

	$3,
	$4,
	$5,
	$6,
	$7,
	$8,

	$9,
	$10
) RETURNING id
`,
		entity.Partition,
		partitionSequence("incident", entity.Partition),

		entity.ElementId,
		entity.ElementInstanceId,
		entity.JobId,
		entity.ProcessId,
		entity.ProcessInstanceId,
		entity.TaskId,

		entity.CreatedAt,
		entity.CreatedBy,
	)

	if err := row.Scan(&entity.Id); err != nil {
		return fmt.Errorf("failed to insert incident %+v: %v", entity, err)
	}

	return nil
}

func (r incidentRepository) Select(partition time.Time, id int32) (*internal.IncidentEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
SELECT
	element_id,
	element_instance_id,
	job_id,
	process_id,
	process_instance_id,
	task_id,

	created_at,
	created_by,
	resolved_at,
	resolved_by
FROM
	incident
WHERE
	partition = $1 AND
	id = $2
FOR UPDATE
`, partition, id)

	var entity internal.IncidentEntity
	if err := row.Scan(
		&entity.ElementId,
		&entity.ElementInstanceId,
		&entity.JobId,
		&entity.ProcessId,
		&entity.ProcessInstanceId,
		&entity.TaskId,

		&entity.CreatedAt,
		&entity.CreatedBy,
		&entity.ResolvedAt,
		&entity.ResolvedBy,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		} else {
			return nil, fmt.Errorf("failed to select incident %s/%d: %v", partition.Format(time.DateOnly), id, err)
		}
	}

	entity.Partition = partition
	entity.Id = id

	return &entity, nil
}

func (r incidentRepository) Update(entity *internal.IncidentEntity) error {
	if _, err := r.tx.Exec(r.txCtx, `
UPDATE
	incident
SET
	resolved_at = $3,
	resolved_by = $4
WHERE
	partition = $1 AND
	id = $2
`,
		entity.Partition,
		entity.Id,

		entity.ResolvedAt,
		entity.ResolvedBy,
	); err != nil {
		return fmt.Errorf("failed to update incident %+v: %v", entity, err)
	}

	return nil
}

func (r incidentRepository) Query(criteria engine.IncidentCriteria, options engine.QueryOptions) ([]engine.Incident, error) {
	var sql bytes.Buffer
	if err := sqlIncidentQuery.Execute(&sql, map[string]any{
		"c": criteria,
		"o": options,
	}); err != nil {
		return nil, fmt.Errorf("failed to execute incident query template: %v", err)
	}

	rows, err := r.tx.Query(r.txCtx, sql.String())
	if err != nil {
		return nil, fmt.Errorf("failed to execute incident query: %v", err)
	}

	defer rows.Close()

	var results []engine.Incident
	for rows.Next() {
		var entity internal.IncidentEntity

		if err := rows.Scan(
			&entity.Partition,
			&entity.Id,

			&entity.ElementId,
			&entity.ElementInstanceId,
			&entity.JobId,
			&entity.ProcessId,
			&entity.ProcessInstanceId,
			&entity.TaskId,

			&entity.CreatedAt,
			&entity.CreatedBy,
			&entity.ResolvedAt,
			&entity.ResolvedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan incident row: %v", err)
		}

		results = append(results, entity.Incident())
	}

	return results, nil
}
