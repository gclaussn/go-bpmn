package pg

import (
	"bytes"
	"context"
	"fmt"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type elementRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r elementRepository) InsertBatch(entities []*internal.ElementEntity) error {
	if len(entities) == 0 {
		return nil
	}

	batch := &pgx.Batch{}

	for _, entity := range entities {
		batch.Queue(`
INSERT INTO element (
	process_id,

	bpmn_element_id,
	bpmn_element_name,
	bpmn_element_type,
	is_multi_instance
) VALUES (
	$1,

	$2,
	$3,
	$4,
	$5
) RETURNING id
`,
			entity.ProcessId,

			entity.BpmnElementId,
			entity.BpmnElementName,
			entity.BpmnElementType.String(),
			entity.IsMultiInstance,
		)
	}

	batchResults := r.tx.SendBatch(r.txCtx, batch)
	defer batchResults.Close()

	for i := range entities {
		row := batchResults.QueryRow()

		if err := row.Scan(&entities[i].Id); err != nil {
			return fmt.Errorf("failed to insert element %+v: %v", entities[i], err)
		}
	}

	return nil
}

func (r elementRepository) SelectByProcessId(processId int32) ([]*internal.ElementEntity, error) {
	rows, err := r.tx.Query(r.txCtx, `
SELECT
	id,

	bpmn_element_id,
	bpmn_element_name,
	bpmn_element_type,
	is_multi_instance
FROM
	element
WHERE
	process_id = $1
`, processId)
	if err != nil {
		return nil, fmt.Errorf("failed to select elements by process ID %d: %v", processId, err)
	}

	defer rows.Close()

	var entities []*internal.ElementEntity
	for rows.Next() {
		var entity internal.ElementEntity

		var bpmnElementTypeValue string

		if err := rows.Scan(
			&entity.Id,

			&entity.BpmnElementId,
			&entity.BpmnElementName,
			&bpmnElementTypeValue,
			&entity.IsMultiInstance,
		); err != nil {
			return nil, fmt.Errorf("failed to scan element row: %v", err)
		}

		entity.ProcessId = processId
		entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)

		entities = append(entities, &entity)
	}

	return entities, nil
}

func (r elementRepository) Query(criteria engine.ElementCriteria, options engine.QueryOptions) ([]engine.Element, error) {
	var sql bytes.Buffer
	if err := sqlElementQuery.Execute(&sql, map[string]any{
		"c": criteria,
		"o": options,
	}); err != nil {
		return nil, fmt.Errorf("failed to execute element query template: %v", err)
	}

	rows, err := r.tx.Query(r.txCtx, sql.String())
	if err != nil {
		return nil, fmt.Errorf("failed to execute element query: %v", err)
	}

	defer rows.Close()

	results := make([]engine.Element, 0)
	for rows.Next() {
		var (
			entity internal.ElementEntity

			bpmnElementTypeValue string

			isSuspended pgtype.Bool

			errorCode    pgtype.Text
			messageName  pgtype.Text
			signalName   pgtype.Text
			time         pgtype.Timestamp
			timeCycle    pgtype.Text
			timeDuration pgtype.Text
		)

		if err := rows.Scan(
			&entity.Id,

			&entity.ProcessId,

			&entity.BpmnElementId,
			&entity.BpmnElementName,
			&bpmnElementTypeValue,
			&entity.IsMultiInstance,

			&isSuspended,

			&errorCode,
			&messageName,
			&signalName,
			&time,
			&timeCycle,
			&timeDuration,
		); err != nil {
			return nil, fmt.Errorf("failed to scan element row: %v", err)
		}

		entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)

		result := entity.Element()
		if isSuspended.Valid {
			var timer *engine.Timer
			switch entity.BpmnElementType {
			case model.ElementTimerCatchEvent, model.ElementTimerStartEvent:
				timer = &engine.Timer{
					Time:         time.Time,
					TimeCycle:    timeCycle.String,
					TimeDuration: engine.ISO8601Duration(timeDuration.String),
				}
			}

			result.EventDefinition = &engine.EventDefinition{
				IsSuspended: isSuspended.Bool,

				ErrorCode:   errorCode.String,
				MessageName: messageName.String,
				SignalName:  signalName.String,
				Timer:       timer,
			}
		}

		results = append(results, result)
	}

	return results, nil
}
