package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVariables(t *testing.T) {
	assert := assert.New(t)

	variables := Variables{}

	// when
	variableA, aExists := variables["a"]

	// then
	assert.False(aExists)
	assert.Empty(variableA.Name)
	assert.Nil(variableA.Value)

	// when
	variables.PutVariable("a", "va")
	variables.PutEncryptedVariable("c", "vc")

	// then
	variableA, aExists = variables["a"]
	assert.True(aExists)
	assert.Empty(variableA.Encoding)
	assert.False(variableA.IsEncrypted)
	assert.Equal("a", variableA.Name)
	assert.Equal("va", variableA.Value)

	variableC, cExists := variables["c"]
	assert.True(cExists)
	assert.Empty(variableC.Encoding)
	assert.True(variableC.IsEncrypted)
	assert.Equal("c", variableC.Name)
	assert.Equal("vc", variableC.Value)

	// when
	variables.DeleteVariable("a")

	// then
	variableA, aExists = variables["a"]
	assert.True(aExists)
	assert.Empty(variableA.Encoding)
	assert.False(variableA.IsEncrypted)
	assert.Equal("a", variableA.Name)
	assert.Nil(variableA.Value)

	// when
	variables.SetVariable(Variable{
		Encoding: "custom",
		Name:     "c",
		Value:    "vc*",
	})

	// then
	variableC, cExists = variables["c"]
	assert.True(cExists)
	assert.Equal("custom", variableC.Encoding)
	assert.False(variableC.IsEncrypted)
	assert.Equal("c", variableC.Name)
	assert.Equal("vc*", variableC.Value)
}
