package pg

import (
	"bytes"
	"context"
	"fmt"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type processRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r processRepository) Insert(entity *internal.ProcessEntity) error {
	row := r.tx.QueryRow(r.txCtx, `
INSERT INTO process (
	bpmn_process_id,
	bpmn_xml,
	bpmn_xml_md5,
	created_at,
	created_by,
	parallelism,
	tags,
	version
) VALUES (
	$1,
	$2,
	$3,
	$4,
	$5,
	$6,
	$7,
	$8
) ON CONFLICT (bpmn_process_id,version) DO NOTHING RETURNING id
`,
		entity.BpmnProcessId,
		entity.BpmnXml,
		entity.BpmnXmlMd5,
		entity.CreatedAt,
		entity.CreatedBy,
		entity.Parallelism,
		entity.Tags,
		entity.Version,
	)

	if err := row.Scan(&entity.Id); err != nil {
		if err == pgx.ErrNoRows { // indicates a conflict
			return err
		} else {
			return fmt.Errorf("failed to insert process %+v: %v", entity, err)
		}
	}

	return nil
}

func (r processRepository) Select(id int32) (*internal.ProcessEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
SELECT
	bpmn_process_id,
	bpmn_xml,
	bpmn_xml_md5,
	created_at,
	created_by,
	parallelism,
	tags,
	version
FROM
	process
WHERE
	id = $1
`, id)

	var entity internal.ProcessEntity
	if err := row.Scan(
		&entity.BpmnProcessId,
		&entity.BpmnXml,
		&entity.BpmnXmlMd5,
		&entity.CreatedAt,
		&entity.CreatedBy,
		&entity.Parallelism,
		&entity.Tags,
		&entity.Version,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		} else {
			return nil, fmt.Errorf("failed to select process %d: %v", id, err)
		}
	}

	entity.Id = id

	return &entity, nil
}

func (r processRepository) SelectByBpmnProcessIdAndVersion(bpmnProcessId string, version string) (*internal.ProcessEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
SELECT
	id,

	bpmn_xml,
	bpmn_xml_md5,
	created_at,
	created_by,
	parallelism,
	tags
FROM
	process
WHERE
	bpmn_process_id = $1 AND
	version = $2
`, bpmnProcessId, version)

	var entity internal.ProcessEntity
	if err := row.Scan(
		&entity.Id,

		&entity.BpmnXml,
		&entity.BpmnXmlMd5,
		&entity.CreatedAt,
		&entity.CreatedBy,
		&entity.Parallelism,
		&entity.Tags,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		} else {
			return nil, fmt.Errorf(
				"failed to select process by BPMN process ID %s and version %s: %v",
				bpmnProcessId,
				version,
				err,
			)
		}
	}

	entity.BpmnProcessId = bpmnProcessId
	entity.Version = version

	return &entity, nil
}

func (r processRepository) Query(criteria engine.ProcessCriteria, options engine.QueryOptions) ([]engine.Process, error) {
	var sql bytes.Buffer
	if err := sqlProcessQuery.Execute(&sql, map[string]any{
		"c": criteria,
		"o": options,
	}); err != nil {
		return nil, fmt.Errorf("failed to execute process query template: %v", err)
	}

	rows, err := r.tx.Query(r.txCtx, sql.String())
	if err != nil {
		return nil, fmt.Errorf("failed to execute process query: %v", err)
	}

	defer rows.Close()

	results := make([]engine.Process, 0)
	for rows.Next() {
		var entity internal.ProcessEntity

		if err := rows.Scan(
			&entity.Id,

			&entity.BpmnProcessId,
			&entity.CreatedAt,
			&entity.CreatedBy,
			&entity.Parallelism,
			&entity.Tags,
			&entity.Version,
		); err != nil {
			return nil, fmt.Errorf("failed to scan process row: %v", err)
		}

		results = append(results, entity.Process())
	}

	return results, nil
}
