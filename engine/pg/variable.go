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

type variableRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r variableRepository) Delete(entity *internal.VariableEntity) error {
	if entity.ElementInstanceId.Valid {
		if _, err := r.tx.Exec(r.txCtx, `
DELETE FROM
	variable
WHERE
	partition = $1 AND
	element_instance_id = $2 AND
	name = $3
`,
			entity.Partition,
			entity.ElementInstanceId,
			entity.Name,
		); err != nil {
			return fmt.Errorf("failed to delete variable %+v by element instance ID: %v", entity, err)
		}
	} else {
		if _, err := r.tx.Exec(r.txCtx, `
DELETE FROM
	variable
WHERE
	partition = $1 AND
	process_instance_id = $2 AND
	element_instance_id IS NULL AND
	name = $3
`,
			entity.Partition,
			entity.ProcessInstanceId,
			entity.Name,
		); err != nil {
			return fmt.Errorf("failed to delete variable %+v by process instance ID: %v", entity, err)
		}
	}
	return nil
}

func (r variableRepository) Insert(entity *internal.VariableEntity) error {
	row := r.tx.QueryRow(r.txCtx, `
INSERT INTO variable (
	partition,
	id,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	created_at,
	created_by,
	encoding,
	is_encrypted,
	name,
	updated_at,
	updated_by,
	value
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
	$10,
	$11,
	$12,
	$13,
	$14
) RETURNING id
`,
		entity.Partition,
		partitionSequence("variable", entity.Partition),

		entity.ElementId,
		entity.ElementInstanceId,
		entity.ProcessId,
		entity.ProcessInstanceId,

		entity.CreatedAt,
		entity.CreatedBy,
		entity.Encoding,
		entity.IsEncrypted,
		entity.Name,
		entity.UpdatedAt,
		entity.UpdatedBy,
		entity.Value,
	)

	if err := row.Scan(&entity.Id); err != nil {
		return fmt.Errorf("failed to insert variable %+v: %v", entity, err)
	}

	return nil
}

func (r variableRepository) SelectByElementInstance(cmd engine.GetElementVariablesCmd) ([]*internal.VariableEntity, error) {
	var sql bytes.Buffer
	if err := sqlVariableElementQuery.Execute(&sql, cmd); err != nil {
		return nil, fmt.Errorf("failed to execute variable element query template: %v", err)
	}

	rows, err := r.tx.Query(r.txCtx, sql.String())
	if err != nil {
		return nil, fmt.Errorf(
			"failed to select variables by element instance %s/%d: %v",
			cmd.Partition,
			cmd.ElementInstanceId,
			err,
		)
	}

	defer rows.Close()

	var entities []*internal.VariableEntity
	for rows.Next() {
		var entity internal.VariableEntity

		if err := rows.Scan(
			&entity.Encoding,
			&entity.IsEncrypted,
			&entity.Name,
			&entity.Value,
		); err != nil {
			return nil, fmt.Errorf("failed to scan variable row: %v", err)
		}

		entities = append(entities, &entity)
	}

	return entities, nil
}

func (r variableRepository) SelectByProcessInstance(cmd engine.GetProcessVariablesCmd) ([]*internal.VariableEntity, error) {
	var sql bytes.Buffer
	if err := sqlVariableProcessQuery.Execute(&sql, cmd); err != nil {
		return nil, fmt.Errorf("failed to execute variable process query template: %v", err)
	}

	rows, err := r.tx.Query(r.txCtx, sql.String())
	if err != nil {
		return nil, fmt.Errorf(
			"failed to select variables by process instance %s/%d: %v",
			cmd.Partition,
			cmd.ProcessInstanceId,
			err,
		)
	}

	defer rows.Close()

	var entities []*internal.VariableEntity
	for rows.Next() {
		var entity internal.VariableEntity

		if err := rows.Scan(
			&entity.Encoding,
			&entity.IsEncrypted,
			&entity.Name,
			&entity.Value,
		); err != nil {
			return nil, fmt.Errorf("failed to scan variable row: %v", err)
		}

		entities = append(entities, &entity)
	}

	return entities, nil
}

func (r variableRepository) Upsert(entity *internal.VariableEntity) error {
	if entity.ElementInstanceId.Valid {
		row := r.tx.QueryRow(r.txCtx, `
SELECT
	id
FROM
	variable
WHERE
	partition = $1 AND
	element_instance_id = $2 AND
	name = $3
		`,
			entity.Partition,
			entity.ElementInstanceId,
			entity.Name,
		)

		err := row.Scan(&entity.Id)
		if err == pgx.ErrNoRows {
			return r.Insert(entity)
		}
		if err != nil {
			return fmt.Errorf(
				"failed to select variable ID by element instance %s/%d and name %s: %v",
				entity.Partition.Format(time.DateOnly),
				entity.Id,
				entity.Name,
				err,
			)
		}
	} else {
		row := r.tx.QueryRow(r.txCtx, `
SELECT
	id
FROM
	variable
WHERE
	partition = $1 AND
	process_instance_id = $2 AND
	name = $3
		`,
			entity.Partition,
			entity.ProcessInstanceId,
			entity.Name,
		)

		err := row.Scan(&entity.Id)
		if err == pgx.ErrNoRows {
			return r.Insert(entity)
		}
		if err != nil {
			return fmt.Errorf(
				"failed to select variable ID by process instance %s/%d and name %s: %v",
				entity.Partition.Format(time.DateOnly),
				entity.Id,
				entity.Name,
				err,
			)
		}
	}

	if _, err := r.tx.Exec(r.txCtx, `
UPDATE
	variable
SET
	encoding = $3,
	is_encrypted = $4,
	name = $5,
	updated_at = $6,
	updated_by = $7,
	value = $8
WHERE
	partition = $1 AND
	id = $2
`,
		entity.Partition,
		entity.Id,

		entity.Encoding,
		entity.IsEncrypted,
		entity.Name,
		entity.UpdatedAt,
		entity.UpdatedBy,
		entity.Value,
	); err != nil {
		return fmt.Errorf("failed to update variable %+v: %v", entity, err)
	}

	return nil
}

func (r variableRepository) Query(criteria engine.VariableCriteria, options engine.QueryOptions) ([]engine.Variable, error) {
	var sql bytes.Buffer
	if err := sqlVariableQuery.Execute(&sql, map[string]any{
		"c": criteria,
		"o": options,
	}); err != nil {
		return nil, fmt.Errorf("failed to execute variable query template: %v", err)
	}

	rows, err := r.tx.Query(r.txCtx, sql.String())
	if err != nil {
		return nil, fmt.Errorf("failed to execute variable query: %v", err)
	}

	defer rows.Close()

	var results []engine.Variable
	for rows.Next() {
		var entity internal.VariableEntity

		if err := rows.Scan(
			&entity.Partition,
			&entity.Id,

			&entity.ElementId,
			&entity.ElementInstanceId,
			&entity.ProcessId,
			&entity.ProcessInstanceId,

			&entity.CreatedAt,
			&entity.CreatedBy,
			&entity.Encoding,
			&entity.IsEncrypted,
			&entity.Name,
			&entity.UpdatedAt,
			&entity.UpdatedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan variable row: %v", err)
		}

		results = append(results, entity.Variable())
	}

	return results, nil
}
