package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestElementVariables(t *testing.T) {
	assert := assert.New(t)

	t.Run("variables", func(t *testing.T) {
		variables := ElementVariables{
			BpmnElementIdMap: map[string]string{"c": "element-c"},
			EncodingMap:      map[string]string{"a": "text", "c": "json"},
			EncryptedMap:     map[string]string{"a": "true"},
			ValueMap:         map[string]string{"a": "b", "x": "", "c": "1"},
		}

		results, err := variables.Variables()
		assert.Nil(err)

		assert.Len(results, 3)

		assert.Empty(results[0].BpmnElementId)
		assert.Equal("a", results[0].Name)
		assert.False(results[0].IsDeleted())
		assert.Equal("text", results[0].Data.Encoding)
		assert.True(results[0].Data.IsEncrypted)
		assert.Equal("b", results[0].Data.Value)

		assert.Equal("element-c", results[1].BpmnElementId)
		assert.Equal("c", results[1].Name)
		assert.False(results[1].IsDeleted())
		assert.Equal("json", results[1].Data.Encoding)
		assert.False(results[1].Data.IsEncrypted)
		assert.Equal("1", results[1].Data.Value)

		assert.Empty(results[2].BpmnElementId)
		assert.Equal("x", results[2].Name)
		assert.True(results[2].IsDeleted())
	})

	t.Run("returns error when encoding is not mapped", func(t *testing.T) {
		variables := ElementVariables{
			EncodingMap: map[string]string{"a": "text"},
		}

		_, err := variables.Variables()
		assert.NotNil(err)
	})

	t.Run("returns error when encrypted is not mapped", func(t *testing.T) {
		variables := ElementVariables{
			EncryptedMap: map[string]string{"a": "true"},
		}

		_, err := variables.Variables()
		assert.NotNil(err)
	})

	t.Run("returns error when encoding is empty", func(t *testing.T) {
		variables := ElementVariables{
			EncodingMap: map[string]string{"a": ""},
			ValueMap:    map[string]string{"a": "b"},
		}

		_, err := variables.Variables()
		assert.NotNil(err)
	})

	t.Run("returns error when encrypted is not a boolean", func(t *testing.T) {
		variables := ElementVariables{
			EncryptedMap: map[string]string{"a": "b"},
			ValueMap:     map[string]string{"a": "b"},
		}

		_, err := variables.Variables()
		assert.NotNil(err)
	})
}

func TestProcessVariables(t *testing.T) {
	assert := assert.New(t)

	t.Run("variables", func(t *testing.T) {
		variables := ProcessVariables{
			EncodingMap:  map[string]string{"a": "text"},
			EncryptedMap: map[string]string{"a": "true"},
			ValueMap:     map[string]string{"a": "b", "x": ""},
		}

		results, err := variables.Variables()
		assert.Nil(err)

		assert.Len(results, 2)

		assert.Equal("a", results[0].Name)
		assert.False(results[0].IsDeleted())
		assert.Equal("text", results[0].Data.Encoding)
		assert.True(results[0].Data.IsEncrypted)
		assert.Equal("b", results[0].Data.Value)

		assert.Equal("x", results[1].Name)
		assert.True(results[1].IsDeleted())
	})

	t.Run("returns error when encoding is not mapped", func(t *testing.T) {
		variables := ProcessVariables{
			EncodingMap: map[string]string{"a": "text"},
		}

		_, err := variables.Variables()
		assert.NotNil(err)
	})

	t.Run("returns error when encrypted is not mapped", func(t *testing.T) {
		variables := ProcessVariables{
			EncryptedMap: map[string]string{"a": "true"},
		}

		_, err := variables.Variables()
		assert.NotNil(err)
	})

	t.Run("returns error when encoding is empty", func(t *testing.T) {
		variables := ProcessVariables{
			EncodingMap: map[string]string{"a": ""},
			ValueMap:    map[string]string{"a": "b"},
		}

		_, err := variables.Variables()
		assert.NotNil(err)
	})

	t.Run("returns error when encrypted is not a boolean", func(t *testing.T) {
		variables := ProcessVariables{
			EncryptedMap: map[string]string{"a": "b"},
			ValueMap:     map[string]string{"a": "b"},
		}

		_, err := variables.Variables()
		assert.NotNil(err)
	})
}

func TestTimer(t *testing.T) {
	assert := assert.New(t)

	var timer Timer
	assert.Nil(timer.Timer())

	now := time.Now()

	timer = Timer{Time: Time(now)}
	assert.NotNil(timer.Timer())
	assert.Equal(now, timer.Timer().Time)

	timer = Timer{TimeCycle: "0 * * * *"}
	assert.NotNil(timer.Timer())
	assert.Equal("0 * * * *", timer.Timer().TimeCycle)

	timer = Timer{TimeDuration: ISO8601Duration("P1D")}
	assert.NotNil(timer.Timer())
	assert.Equal("P1D", timer.Timer().TimeDuration.String())
}
