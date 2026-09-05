package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/adhocore/gronx"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
)

func evaluateTimer(timer engine.Timer, start time.Time) (time.Time, error) {
	switch {
	case !timer.Time.IsZero():
		// must be UTC and truncated to millis (see engine/pg/pg.go:pgEngineWithContext#require)
		return timer.Time.UTC().Truncate(time.Millisecond), nil
	case timer.TimeCycle != "":
		return gronx.NextTickAfter(timer.TimeCycle, start, false)
	case !timer.TimeDuration.IsZero():
		return timer.TimeDuration.Calculate(start), nil
	default:
		return time.Time{}, errors.New("must specify a time, time cycle or time duration")
	}
}

func elementPointer(bpmnElement *model.Element) string {
	var ids []string

	curr := bpmnElement
	for {
		ids = append(ids, curr.Id)
		if curr.Parent == nil {
			break
		}
		curr = curr.Parent
	}

	ids = append(ids, "") // for leading slash

	slices.Reverse(ids)

	return strings.Join(ids, "/")
}

// isTaskOrScope determines if a element type is a task or a scope, to which boundary events can be attached.
func isTaskOrScope(elementType model.ElementType) bool {
	switch elementType {
	case
		model.ElementBusinessRuleTask,
		model.ElementCallActivity,
		model.ElementScriptTask,
		model.ElementSendTask,
		model.ElementServiceTask,
		model.ElementSubProcess,
		model.ElementUserTask:
		return true
	default:
		return false
	}
}

func isTimerEvent(elementType model.ElementType) bool {
	switch elementType {
	case
		model.ElementTimerBoundaryEvent,
		model.ElementTimerCatchEvent,
		model.ElementTimerStartEvent:
		return true
	default:
		return false
	}
}

func marshalTags(tags []engine.Tag) (pgtype.Text, error) {
	if len(tags) == 0 {
		return pgtype.Text{}, nil
	}

	tagMap := make(map[string]string, len(tags))
	for _, tag := range tags {
		if tag.Value != "" {
			tagMap[tag.Name] = tag.Value
		} else {
			delete(tagMap, tag.Name)
		}
	}

	b, err := json.Marshal(tagMap)
	if err != nil {
		return pgtype.Text{}, fmt.Errorf("failed to marshal tags: %v", err)
	}

	return pgtype.Text{String: string(b), Valid: true}, nil
}

func unmarshalTags(value pgtype.Text) []engine.Tag {
	if !value.Valid {
		return nil
	}

	var tagMap map[string]string
	_ = json.Unmarshal([]byte(value.String), &tagMap)

	tagNames := slices.Sorted(maps.Keys(tagMap))

	tags := make([]engine.Tag, len(tagNames))
	for i, tagName := range tagNames {
		tags[i] = engine.Tag{
			Name:  tagName,
			Value: tagMap[tagName],
		}
	}

	return tags
}
