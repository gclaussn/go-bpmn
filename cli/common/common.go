package common

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

const (
	AnnotationEngine = "engine" // annotates commands that require an engine
)

var (
	AnnotationEngineMap = map[string]string{AnnotationEngine: ""}
)

func FlagQueryOptions(cmd *cobra.Command, options *engine.QueryOptions) {
	cmd.Flags().IntVar(&options.Limit, "limit", 100, "Maximum number of results to return")
	cmd.Flags().IntVar(&options.Offset, "offset", 0, "Number of results to skip, before returning any result")
}

func GetEngine(cmd *cobra.Command) engine.Engine {
	ctx := cmd.Context()

	value := ctx.Value(engineKey{})
	if value == nil {
		return nil
	}

	return value.(engine.Engine)
}

func GetEngineAndWorkerId(cmd *cobra.Command) (engine.Engine, string) {
	ctx := cmd.Context()
	return ctx.Value(engineKey{}).(engine.Engine), ctx.Value(workerIdKey{}).(string)
}

func Help(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

func SetEngineAndWorkerId(ctx context.Context, e engine.Engine, workerId string) context.Context {
	return context.WithValue(context.WithValue(ctx, engineKey{}, e), workerIdKey{}, workerId)
}

func Tags(tagMap map[string]string) []engine.Tag {
	tags := make([]engine.Tag, 0, len(tagMap))
	for name, value := range tagMap {
		tags = append(tags, engine.Tag{
			Name:  name,
			Value: value,
		})
	}

	slices.SortFunc(tags, func(a engine.Tag, b engine.Tag) int {
		return strings.Compare(a.Name, b.Name)
	})

	return tags
}

type ElementVariables struct {
	BpmnElementIdMap map[string]string
	EncodingMap      map[string]string
	EncryptedMap     map[string]string
	ValueMap         map[string]string
}

func (v ElementVariables) Variables() ([]engine.ElementVariable, error) {
	for name := range v.EncodingMap {
		if _, ok := v.ValueMap[name]; !ok {
			return nil, fmt.Errorf("variable %s is not set, but encoding is", name)
		}
	}
	for name := range v.EncryptedMap {
		if _, ok := v.ValueMap[name]; !ok {
			return nil, fmt.Errorf("variable %s is not set, but encrypted is", name)
		}
	}

	variables := make([]engine.ElementVariable, 0, len(v.ValueMap))
	for name, value := range v.ValueMap {
		if value == "" {
			variables = append(variables, engine.ElementVariable{
				BpmnElementId: v.BpmnElementIdMap[name],
				Name:          name,
			})
			continue
		}

		encoding := v.EncodingMap[name]
		if encoding == "" {
			return nil, fmt.Errorf("variable %s: encoding is empty", name)
		}

		var isEncrypted bool
		encrypted, ok := v.EncryptedMap[name]
		if ok {
			b, err := strconv.ParseBool(encrypted)
			if err != nil {
				return nil, fmt.Errorf("variable %s: encrypted %s is not a boolean", name, encrypted)
			}
			isEncrypted = b
		}

		variables = append(variables, engine.ElementVariable{
			BpmnElementId: v.BpmnElementIdMap[name],
			Name:          name,
			Data: &engine.Data{
				Encoding:    encoding,
				IsEncrypted: isEncrypted,
				Value:       value,
			},
		})
	}

	slices.SortFunc(variables, func(a engine.ElementVariable, b engine.ElementVariable) int {
		if a.Name != b.Name {
			return strings.Compare(a.Name, b.Name)
		} else {
			return strings.Compare(a.BpmnElementId, b.BpmnElementId)
		}
	})

	return variables, nil
}

type ProcessVariables struct {
	EncodingMap  map[string]string
	EncryptedMap map[string]string
	ValueMap     map[string]string
}

func (v ProcessVariables) Variables() ([]engine.ProcessVariable, error) {
	for name := range v.EncodingMap {
		if _, ok := v.ValueMap[name]; !ok {
			return nil, fmt.Errorf("variable %s is not set, but encoding is", name)
		}
	}
	for name := range v.EncryptedMap {
		if _, ok := v.ValueMap[name]; !ok {
			return nil, fmt.Errorf("variable %s is not set, but encrypted is", name)
		}
	}

	variables := make([]engine.ProcessVariable, 0, len(v.ValueMap))
	for name, value := range v.ValueMap {
		if value == "" {
			variables = append(variables, engine.ProcessVariable{
				Name: name,
			})
			continue
		}

		encoding := v.EncodingMap[name]
		if encoding == "" {
			return nil, fmt.Errorf("variable %s: encoding is empty", name)
		}

		var isEncrypted bool
		encrypted, ok := v.EncryptedMap[name]
		if ok {
			b, err := strconv.ParseBool(encrypted)
			if err != nil {
				return nil, fmt.Errorf("variable %s: encrypted %s is not a boolean", name, encrypted)
			}
			isEncrypted = b
		}

		variables = append(variables, engine.ProcessVariable{
			Name: name,
			Data: &engine.Data{
				Encoding:    encoding,
				IsEncrypted: isEncrypted,
				Value:       value,
			},
		})
	}

	slices.SortFunc(variables, func(a engine.ProcessVariable, b engine.ProcessVariable) int {
		return strings.Compare(a.Name, b.Name)
	})

	return variables, nil
}

type Timer struct {
	Time         Time
	TimeCycle    string
	TimeDuration ISO8601Duration
}

func (v Timer) Timer() *engine.Timer {
	timer := engine.Timer{
		Time:         v.Time.Time(),
		TimeCycle:    v.TimeCycle,
		TimeDuration: engine.ISO8601Duration(v.TimeDuration),
	}

	if timer.Time != nil || timer.TimeCycle != "" || !timer.TimeDuration.IsZero() {
		return &timer
	}

	return nil
}

// engineKey is used as context value key.
type engineKey struct{}

// workerIdKey is used as context value key.
type workerIdKey struct{}
