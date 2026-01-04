package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/gclaussn/go-bpmn/http/common"
	"github.com/stretchr/testify/assert"
)

func TestGenerator(t *testing.T) {
	assert := assert.New(t)

	generator := newGenerator("../..")

	t.Run("getPathConsts", func(t *testing.T) {
		pathConsts := generator.getPathConsts()
		assert.NotEmpty(pathConsts)
	})

	t.Run("generateOperations", func(t *testing.T) {
		generator.generateOperations()
	})
}

func TestDescribeEnum(t *testing.T) {
	assert := assert.New(t)

	problemType, problemTypeValues := describeEnum(common.ProblemType(0))
	assert.Equal("common.ProblemType", problemType)
	assert.NotEmpty(problemTypeValues)
	assert.Equal("UNKNOWN", problemTypeValues[len(problemTypeValues)-1])
}

func TestExtractType(t *testing.T) {
	assert := assert.New(t)

	var typeName string

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, "../../engine/command.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	t.Run("simple type", func(t *testing.T) {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.TypeSpec:
				return n.Name.Name == "CreateProcessCmd"
			case *ast.Field:
				if n.Names[0].Name == "Parallelism" {
					typeName = extractType(n.Type)
					return false
				}
			}
			return true
		})

		assert.Equal("int", typeName)
	})

	t.Run("imported type", func(t *testing.T) {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.TypeSpec:
				return n.Name.Name == "SetTimeCmd"
			case *ast.Field:
				if n.Names[0].Name == "Time" {
					typeName = extractType(n.Type)
					return false
				}
			}
			return true
		})

		assert.Equal("time.Time", typeName)
	})

	t.Run("pointer type", func(t *testing.T) {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.TypeSpec:
				return n.Name.Name == "CompleteJobCmd"
			case *ast.Field:
				if n.Names[0].Name == "Completion" {
					typeName = extractType(n.Type)
					return false
				}
			}
			return true
		})

		assert.Equal("JobCompletion", typeName)
	})

	t.Run("slice type", func(t *testing.T) {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.TypeSpec:
				return n.Name.Name == "LockJobsCmd"
			case *ast.Field:
				if n.Names[0].Name == "ProcessIds" {
					typeName = extractType(n.Type)
					return false
				}
			}
			return true
		})

		assert.Equal("[]int32", typeName)
	})
}

func TestParseStructFieldTags(t *testing.T) {
	assert := assert.New(t)

	var (
		json     []string
		validate []string
	)

	json, validate = parseStructFieldTags("")
	assert.Empty(json)
	assert.Empty(validate)

	json, validate = parseStructFieldTags("`json:\"x\"`")
	assert.Equal([]string{"x"}, json)
	assert.Empty(validate)

	json, validate = parseStructFieldTags("`json:\"x,omitempty\"`")
	assert.Equal([]string{"x", "omitempty"}, json)
	assert.Empty(validate)

	json, validate = parseStructFieldTags("`json:\"y,omitempty\" validate:\"gte=1,lte=100\"`")
	assert.Equal([]string{"y", "omitempty"}, json)
	assert.Equal([]string{"gte=1", "lte=100"}, validate)
}
