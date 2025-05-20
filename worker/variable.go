package worker

import (
	"fmt"
)

type Variable struct {
	Encoding    string
	IsEncrypted bool
	Name        string
	Value       any
}

func (v Variable) Decode(jc JobContext, value any) error {
	decoder := jc.worker.Decoder(v.Encoding)
	if decoder == nil {
		return fmt.Errorf("no decoder for encoding %s registered", v.Encoding)
	}

	if s, ok := v.Value.(string); ok {
		return decoder.Decode(s, value)
	} else {
		return nil
	}
}

func (v Variable) IsDeleted() bool {
	return v.Value == nil
}

// Variables is used to get and set variables of a process instance or element instance.
type Variables map[string]Variable

func (v Variables) DeleteVariable(name string) {
	v[name] = Variable{Name: name}
}

func (v Variables) PutVariable(name string, value any) {
	if name != "" {
		v[name] = Variable{Name: name, Value: value}
	}
}

func (v Variables) PutEncryptedVariable(name string, value any) {
	if name != "" {
		v[name] = Variable{Name: name, Value: value, IsEncrypted: true}
	}
}

func (v Variables) SetVariable(variable Variable) {
	if variable.Name != "" {
		v[variable.Name] = variable
	}
}
