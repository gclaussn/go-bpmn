package worker

import (
	"errors"
	"fmt"
)

func NewElementVariables(bpmnElementId string) ElementVariables {
	return ElementVariables{bpmnElementId: bpmnElementId}
}

func NewProcessVariables() ProcessVariables {
	return ProcessVariables{}
}

// ElementVariable is a variable at element instance scope.
type ElementVariable struct {
	BpmnElementId string
	Encoding      string
	IsEncrypted   bool
	Name          string
	Value         any
}

// Decode decodes a variable in a value.
// If decoding fails, an error is returned.
//
// Decode uses the decoder, registered for the variable's encoding.
// If no such decoder is registered, an error is returned.
func (v ElementVariable) Decode(jc JobContext, value any) error {
	if value == nil {
		return errors.New("value is nil")
	}

	decoder := jc.w.Decoder(v.Encoding)
	if decoder == nil {
		return fmt.Errorf("no decoder for encoding %s registered", v.Encoding)
	}

	if s, ok := v.Value.(string); ok {
		return decoder.Decode(s, value)
	} else {
		return nil
	}
}

// IsDeleted determines if a variable is marked for deletion.
func (v ElementVariable) IsDeleted() bool {
	return v.Value == nil
}

// ElementVariables is used to get and set variables at element instance scope.
type ElementVariables struct {
	bpmnElementId string
	variables     []ElementVariable
}

// Delete marks an element variable for deletion.
func (v *ElementVariables) Delete(name string) {
	v.DeleteAt(v.bpmnElementId, name)
}

// DeleteAt marks an element variable at a specific scope for deletion.
func (v *ElementVariables) DeleteAt(bpmnElementId string, name string) {
	v.PutAt(bpmnElementId, name, nil)
}

// Get returns the element variable with the given name and a success indicator.
func (v *ElementVariables) Get(name string) (ElementVariable, bool) {
	return v.GetAt(v.bpmnElementId, name)
}

// GetAt returns the element variable with the given name at a specific scope and a success indicator.
func (v *ElementVariables) GetAt(bpmnElementId string, name string) (ElementVariable, bool) {
	if i := v.indexOf(bpmnElementId, name); i >= 0 {
		return v.variables[i], true
	} else {
		return ElementVariable{}, false
	}
}

// Put adds or replaces an element variable.
func (v *ElementVariables) Put(name string, value any) {
	v.PutAt(v.bpmnElementId, name, value)
}

// PutAt adds or replaces an element variable at a specific scope.
func (v *ElementVariables) PutAt(bpmnElementId string, name string, value any) {
	if i := v.indexOf(bpmnElementId, name); i >= 0 {
		v.variables[i] = ElementVariable{
			BpmnElementId: bpmnElementId,
			Name:          name,
			Value:         value,
		}
	} else {
		v.variables = append(v.variables, ElementVariable{
			BpmnElementId: bpmnElementId,
			Name:          name,
			Value:         value,
		})
	}
}

// PutEncrypted adds or replaces an encrypted element variable.
func (v *ElementVariables) PutEncrypted(name string, value any) {
	v.PutEncryptedAt(v.bpmnElementId, name, value)
}

// PutEncryptedAt adds or replaces an encrypted element variable at a specific scope.
func (v *ElementVariables) PutEncryptedAt(bpmnElementId string, name string, value any) {
	if i := v.indexOf(bpmnElementId, name); i >= 0 {
		v.variables[i] = ElementVariable{
			BpmnElementId: bpmnElementId,
			IsEncrypted:   true,
			Name:          name,
			Value:         value,
		}
	} else {
		v.variables = append(v.variables, ElementVariable{
			BpmnElementId: bpmnElementId,
			IsEncrypted:   true,
			Name:          name,
			Value:         value,
		})
	}
}

// Set sets a given element variable.
func (v *ElementVariables) Set(variable ElementVariable) {
	if variable.BpmnElementId == "" {
		variable.BpmnElementId = v.bpmnElementId
	}

	if i := v.indexOf(variable.BpmnElementId, variable.Name); i >= 0 {
		v.variables[i] = variable
	} else {
		v.variables = append(v.variables, variable)
	}
}

func (v *ElementVariables) indexOf(bpmnElementId string, name string) int {
	for i, variable := range v.variables {
		if variable.BpmnElementId == bpmnElementId && variable.Name == name {
			return i
		}
	}
	return -1
}

// ProcessVariable is a variable at process instance scope.
type ProcessVariable struct {
	Encoding    string
	IsEncrypted bool
	Name        string
	Value       any
}

// Decode decodes a variable in a value.
// If decoding fails, an error is returned.
//
// Decode uses the decoder, registered for the variable's encoding.
// If no such decoder is registered, an error is returned.
func (v ProcessVariable) Decode(jc JobContext, value any) error {
	if value == nil {
		return errors.New("value is nil")
	}

	decoder := jc.w.Decoder(v.Encoding)
	if decoder == nil {
		return fmt.Errorf("no decoder for encoding %s registered", v.Encoding)
	}

	if s, ok := v.Value.(string); ok {
		return decoder.Decode(s, value)
	} else {
		return nil
	}
}

// IsDeleted determines if a variable is marked for deletion.
func (v ProcessVariable) IsDeleted() bool {
	return v.Value == nil
}

// ProcessVariables is used to get and set variables at process instance scope.
type ProcessVariables struct {
	variables []ProcessVariable
}

// Delete marks a process variable for deletion.
func (v *ProcessVariables) Delete(name string) {
	v.Put(name, nil)
}

// Get returns the process variable with the given name and a success indicator.
func (v *ProcessVariables) Get(name string) (ProcessVariable, bool) {
	if i := v.indexOf(name); i >= 0 {
		return v.variables[i], true
	} else {
		return ProcessVariable{}, false
	}
}

// Put adds or replaces a process variable.
func (v *ProcessVariables) Put(name string, value any) {
	if i := v.indexOf(name); i >= 0 {
		v.variables[i] = ProcessVariable{
			Name:  name,
			Value: value,
		}
	} else {
		v.variables = append(v.variables, ProcessVariable{
			Name:  name,
			Value: value,
		})
	}
}

// PutEncrypted adds or replaces an encrypted process variable.
func (v *ProcessVariables) PutEncrypted(name string, value any) {
	if i := v.indexOf(name); i >= 0 {
		v.variables[i] = ProcessVariable{
			IsEncrypted: true,
			Name:        name,
			Value:       value,
		}
	} else {
		v.variables = append(v.variables, ProcessVariable{
			IsEncrypted: true,
			Name:        name,
			Value:       value,
		})
	}
}

// Set sets a given process variable.
func (v *ProcessVariables) Set(variable ProcessVariable) {
	if i := v.indexOf(variable.Name); i >= 0 {
		v.variables[i] = variable
	} else {
		v.variables = append(v.variables, variable)
	}
}

func (v *ProcessVariables) indexOf(name string) int {
	for i, variable := range v.variables {
		if variable.Name == name {
			return i
		}
	}
	return -1
}
