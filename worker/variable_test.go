package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestElementVariables(t *testing.T) {
	assert := assert.New(t)

	variables := ElementVariables{bpmnElementId: "x"}

	// when
	variableA, aExists := variables.Get("a")

	// then
	assert.False(aExists)
	assert.Empty(variableA.Name)
	assert.Nil(variableA.Value)

	// when
	variableA, aExists = variables.GetAt("y", "a")

	// then
	assert.False(aExists)
	assert.Empty(variableA.Name)
	assert.Nil(variableA.Value)

	// when
	variables.Put("a", "va")
	variables.PutEncrypted("c", "vc")

	// then
	variableA, aExists = variables.Get("a")
	assert.True(aExists)
	assert.Equal("x", variableA.BpmnElementId)
	assert.Empty(variableA.Encoding)
	assert.False(variableA.IsEncrypted)
	assert.Equal("a", variableA.Name)
	assert.Equal("va", variableA.Value)

	variableC, cExists := variables.Get("c")
	assert.True(cExists)
	assert.Equal("x", variableC.BpmnElementId)
	assert.Empty(variableC.Encoding)
	assert.True(variableC.IsEncrypted)
	assert.Equal("c", variableC.Name)
	assert.Equal("vc", variableC.Value)

	// when
	variables.PutAt("y", "a", "va")
	variables.PutEncryptedAt("y", "c", "vc")

	// then
	variableA, aExists = variables.GetAt("y", "a")
	assert.True(aExists)
	assert.Equal("y", variableA.BpmnElementId)
	assert.Empty(variableA.Encoding)
	assert.False(variableA.IsEncrypted)
	assert.Equal("a", variableA.Name)
	assert.Equal("va", variableA.Value)

	variableC, cExists = variables.GetAt("y", "c")
	assert.True(cExists)
	assert.Equal("y", variableC.BpmnElementId)
	assert.Empty(variableC.Encoding)
	assert.True(variableC.IsEncrypted)
	assert.Equal("c", variableC.Name)
	assert.Equal("vc", variableC.Value)

	// when
	variables.Delete("a")
	variables.DeleteAt("y", "a")

	// then
	variableA, aExists = variables.Get("a")
	assert.True(aExists)
	assert.True(variableA.IsDeleted())

	assert.Equal("x", variableA.BpmnElementId)
	assert.Empty(variableA.Encoding)
	assert.False(variableA.IsEncrypted)
	assert.Equal("a", variableA.Name)
	assert.Nil(variableA.Value)

	variableA, aExists = variables.GetAt("y", "a")
	assert.True(aExists)
	assert.True(variableA.IsDeleted())

	assert.Equal("y", variableA.BpmnElementId)
	assert.Empty(variableA.Encoding)
	assert.False(variableA.IsEncrypted)
	assert.Equal("a", variableA.Name)
	assert.Nil(variableA.Value)

	// when
	variables.Set(ElementVariable{
		Encoding: "custom",
		Name:     "c",
		Value:    "vc*",
	})

	variables.Set(ElementVariable{
		BpmnElementId: "y",
		Encoding:      "custom",
		Name:          "c",
		Value:         "vc*",
	})

	// then
	variableC, cExists = variables.Get("c")
	assert.True(cExists)
	assert.Equal("x", variableC.BpmnElementId)
	assert.Equal("custom", variableC.Encoding)
	assert.False(variableC.IsEncrypted)
	assert.Equal("c", variableC.Name)
	assert.Equal("vc*", variableC.Value)

	variableC, cExists = variables.GetAt("y", "c")
	assert.True(cExists)
	assert.Equal("y", variableC.BpmnElementId)
	assert.Equal("custom", variableC.Encoding)
	assert.False(variableC.IsEncrypted)
	assert.Equal("c", variableC.Name)
	assert.Equal("vc*", variableC.Value)
}

func TestProcessVariables(t *testing.T) {
	assert := assert.New(t)

	variables := ProcessVariables{}

	// when
	variableA, aExists := variables.Get("a")

	// then
	assert.False(aExists)
	assert.Empty(variableA.Name)
	assert.Nil(variableA.Value)

	// when
	variables.Put("a", "va")
	variables.PutEncrypted("c", "vc")

	// then
	variableA, aExists = variables.Get("a")
	assert.True(aExists)
	assert.Empty(variableA.Encoding)
	assert.False(variableA.IsEncrypted)
	assert.Equal("a", variableA.Name)
	assert.Equal("va", variableA.Value)

	variableC, cExists := variables.Get("c")
	assert.True(cExists)
	assert.Empty(variableC.Encoding)
	assert.True(variableC.IsEncrypted)
	assert.Equal("c", variableC.Name)
	assert.Equal("vc", variableC.Value)

	// when
	variables.Delete("a")

	// then
	variableA, aExists = variables.Get("a")
	assert.True(aExists)
	assert.True(variableA.IsDeleted())

	assert.Empty(variableA.Encoding)
	assert.False(variableA.IsEncrypted)
	assert.Equal("a", variableA.Name)
	assert.Nil(variableA.Value)

	// when
	variables.Set(ProcessVariable{
		Encoding: "custom",
		Name:     "c",
		Value:    "vc*",
	})

	// then
	variableC, cExists = variables.Get("c")
	assert.True(cExists)
	assert.Equal("custom", variableC.Encoding)
	assert.False(variableC.IsEncrypted)
	assert.Equal("c", variableC.Name)
	assert.Equal("vc*", variableC.Value)
}
