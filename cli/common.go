package cli

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func flagQueryOptions(c *cobra.Command, options *engine.QueryOptions) {
	c.Flags().IntVar(&options.Limit, "limit", 100, "")
	c.Flags().IntVar(&options.Offset, "offset", 0, "")
}

func mapElementVariables(
	bpmnElementIdMap map[string]string,
	encodingMap map[string]string,
	encryptedMap map[string]string,
	valueMap map[string]string,
) ([]engine.ElementVariable, error) {
	for name := range encodingMap {
		if _, ok := valueMap[name]; !ok {
			return nil, fmt.Errorf("variable %s: no value defined", name)
		}
	}
	for name := range encryptedMap {
		if _, ok := valueMap[name]; !ok {
			return nil, fmt.Errorf("variable %s: no value defined", name)
		}
	}

	variables := make([]engine.ElementVariable, 0, len(valueMap))
	for name, value := range valueMap {
		if value == "" {
			variables = append(variables, engine.ElementVariable{
				BpmnElementId: bpmnElementIdMap[name],
				Name:          name,
			})
			continue
		}

		var isEncrypted bool
		encrypted, ok := encryptedMap[name]
		if ok {
			b, err := strconv.ParseBool(encrypted)
			if err != nil {
				return nil, fmt.Errorf("variable %s: encrypted value %s is not a boolean", name, encrypted)
			}
			isEncrypted = b
		}

		variables = append(variables, engine.ElementVariable{
			BpmnElementId: bpmnElementIdMap[name],
			Name:          name,
			Data: &engine.Data{
				Encoding:    encodingMap[name],
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

func mapProcessVariables(encodingMap map[string]string, encryptedMap map[string]string, valueMap map[string]string) ([]engine.ProcessVariable, error) {
	for name := range encodingMap {
		if _, ok := valueMap[name]; !ok {
			return nil, fmt.Errorf("variable %s: no value defined", name)
		}
	}
	for name := range encryptedMap {
		if _, ok := valueMap[name]; !ok {
			return nil, fmt.Errorf("variable %s: no value defined", name)
		}
	}

	variables := make([]engine.ProcessVariable, 0, len(valueMap))
	for name, value := range valueMap {
		if value == "" {
			variables = append(variables, engine.ProcessVariable{
				Name: name,
			})
			continue
		}

		var isEncrypted bool
		encrypted, ok := encryptedMap[name]
		if ok {
			b, err := strconv.ParseBool(encrypted)
			if err != nil {
				return nil, fmt.Errorf("variable %s: encrypted value %s is not a boolean", name, encrypted)
			}
			isEncrypted = b
		}

		variables = append(variables, engine.ProcessVariable{
			Name: name,
			Data: &engine.Data{
				Encoding:    encodingMap[name],
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
