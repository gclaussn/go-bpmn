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

func mapVariables(encodingMap map[string]string, encryptedMap map[string]string, valueMap map[string]string) ([]engine.VariableData, error) {
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

	variables := make([]engine.VariableData, 0, len(valueMap))
	for name, value := range valueMap {
		if value == "" {
			variables = append(variables, engine.VariableData{
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

		variables = append(variables, engine.VariableData{
			Name: name,
			Data: &engine.Data{
				Encoding:    encodingMap[name],
				IsEncrypted: isEncrypted,
				Value:       value,
			},
		})
	}

	slices.SortFunc(variables, func(a engine.VariableData, b engine.VariableData) int {
		return strings.Compare(a.Name, b.Name)
	})

	return variables, nil
}
