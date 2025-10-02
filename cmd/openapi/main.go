/*
openapi generates the API documentation.

Following two program arguments are required:

1. A base path to the go-bpmn module directory.

2. An output path for the generated YAML file.
*/
package main

import (
	"bufio"
	"bytes"
	"embed"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/common"
	"github.com/gclaussn/go-bpmn/http/server"
	"github.com/gclaussn/go-bpmn/model"
)

//go:embed templates
var resources embed.FS

func main() {
	log.SetFlags(0)

	flags := flag.NewFlagSet("openapi", flag.ContinueOnError)
	flags.SetOutput(log.Writer())

	var sourcePath string
	flags.StringVar(&sourcePath, "source-path", ".", "base path of the source files")
	var outputPath string
	flags.StringVar(&outputPath, "output-path", ".", "path to the output file")
	var version string
	flags.StringVar(&version, "version", "unknown-version", "API version")

	if err := flags.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}

	if outputPath == "" {
		log.Fatal("please provide an output path")
	}

	generator := newGenerator(sourcePath)

	// model
	elementType, elementTypeValues := describeEnum(model.ElementType(0))
	generator.generateEnum(elementType, elementTypeValues)
	generator.generateSchemas("model/element_type.go")

	// engine
	instanceState, instanceStateValues := describeEnum(engine.InstanceState(0))
	generator.generateEnum(instanceState, instanceStateValues)
	jobType, jobTypeValues := describeEnum(engine.JobType(0))
	generator.generateEnum(jobType, jobTypeValues)
	taskType, taskTypeValues := describeEnum(engine.TaskType(0))
	generator.generateEnum(taskType, taskTypeValues)
	generator.generateSchemas("engine/command.go")
	generator.generateSchemas("engine/model.go")

	// server
	problemType, problemTypeValues := describeEnum(common.ProblemType(0))
	generator.generateEnum(problemType, problemTypeValues)
	generator.generateSchemas("http/common/problem.go")
	generator.generateSchemas("http/common/response.go")

	generator.generateOperations()

	yaml := generator.generateYaml(version)

	outputFile, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		log.Fatalf("failed to open output file: %v", err)
	}

	defer outputFile.Close()

	_, err = outputFile.WriteString(yaml)
	if err != nil {
		log.Fatalf("failed to write output file: %v", err)
	}
}

func newGenerator(sourcePath string) *generator {
	return &generator{
		sourcePath: sourcePath,
		fileSet:    token.NewFileSet(),

		paths:   make(map[string]map[string]*Operation),
		schemas: make(map[string]*Schema),
	}
}

type generator struct {
	sourcePath string
	fileSet    *token.FileSet

	paths   map[string]map[string]*Operation
	schemas map[string]*Schema
}

func (g *generator) generateEnum(typeName string, values []string) {
	g.schemas[typeName] = &Schema{
		Type: "string",
		Enum: values,
	}
}

func (g *generator) generateOperations() {
	f, err := parser.ParseFile(g.fileSet, g.sourcePath+"/http/server/server.go", nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	var (
		newFuncDecl *ast.FuncDecl
		startPos    int
		endPos      int
	)

	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Name.Name == "New" {
				newFuncDecl = d
				break
			}
		}
	}
	if newFuncDecl == nil {
		log.Fatal("failed to find New function")
	}

	// find relevant range within New function
	for _, commentGroup := range f.Comments {
		for _, comment := range commentGroup.List {
			if startPos == 0 && comment.Text == "// operations:start" {
				startPos = int(comment.Slash)
			}
			if endPos == 0 && comment.Text == "// operations:end" {
				endPos = int(comment.Slash)
			}
		}
	}

	if startPos == 0 || endPos == 0 {
		log.Fatal("failed to find start or end position of operations")
	}

	var tokens []string
	for _, statement := range newFuncDecl.Body.List {
		if int(statement.Pos()) < startPos || int(statement.End()) > endPos {
			continue
		}

		ast.Inspect(statement, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.BasicLit:
				tokens = append(tokens, n.Value)
			case *ast.Ident:
				tokens = append(tokens, n.Name)
			}
			return true
		})
	}

	pathConsts := g.getPathConsts()

	for i := 0; i < len(tokens); i += 7 {
		// e.g. ["mux", "HandleFunc", "\"POST \"", "common", "PathElementsQuery", "server", "queryElements"]

		method := strings.ToLower(tokens[i+2][1 : len(tokens[i+2])-2]) // e.g. `"POST "` -> "post"
		pathConst := tokens[i+4]                                       // e.g. "PathElementsQuery"
		operationId := tokens[i+6]                                     // e.g. "queryElements"

		path, ok := pathConsts[pathConst]
		if !ok {
			log.Fatalf("failed to look up path const %s", pathConst)
		}

		methods, ok := g.paths[path]
		if !ok {
			methods = make(map[string]*Operation)
			g.paths[path] = methods
		}

		var parameters []string

		// extract path parameters
		var j int
		for i, r := range path {
			switch r {
			case '{':
				j = i
			case '}':
				parameters = append(parameters, path[j+1:i])
			}
		}

		// set query parameters
		if strings.HasPrefix(operationId, "query") {
			parameters = append(parameters, common.QueryOffset)
			parameters = append(parameters, common.QueryLimit)
		}

		if path == common.PathElementInstancesVariables || path == common.PathProcessInstancesVariables {
			if method == "get" {
				parameters = append(parameters, common.QueryNames)
			}
		}

		methods[method] = &Operation{
			Id:         operationId,
			Parameters: parameters,
		}
	}
}

func (g *generator) generateSchemas(name string) {
	f, err := parser.ParseFile(g.fileSet, g.sourcePath+"/"+name, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	packageName := f.Name.Name

	var (
		typeName    string
		schema      *Schema
		description string
	)

	for _, d := range f.Decls {
		ast.Inspect(d, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.GenDecl:
				if n.Doc != nil {
					description = n.Doc.Text()
				} else {
					description = ""
				}
			case *ast.TypeSpec:
				typeName = fmt.Sprintf("%s.%s", packageName, n.Name.Name)

				schema = g.schemas[typeName]
				if schema == nil {
					schema = &Schema{Type: extractType(n.Type)}
					g.schemas[typeName] = schema
				}

				if description != "" {
					schema.Description = description[:len(description)-1]
				}
			case *ast.StructType:
				schema.Type = "object"

				for _, field := range n.Fields.List {
					if field.Tag == nil {
						continue
					}

					json, validate := parseStructFieldTags(field.Tag.Value)
					if len(json) == 0 {
						continue
					}

					typeName := extractType(field.Type)

					// when type is exported, prepend the package name
					typeNameRunes := []rune(typeName)
					if unicode.IsUpper(typeNameRunes[0]) {
						typeName = fmt.Sprintf("%s.%s", packageName, typeName)
					} else if strings.HasPrefix(typeName, "[]") && unicode.IsUpper(typeNameRunes[2]) {
						typeName = fmt.Sprintf("[]%s.%s", packageName, typeName[2:])
					}

					property := Property{
						Name: json[0],

						Maximum: -1,
						Minimum: -1,

						typeName: typeName,
						validate: validate,
					}

					var description string
					if field.Comment != nil {
						description = field.Comment.Text()
					} else if field.Doc != nil {
						description = field.Doc.Text()
					}

					if description != "" {
						property.Description = description[:len(description)-1]
					}

					schema.Properties = append(schema.Properties, &property)
				}

				// delete empty schemas e.g. GetBpmnXmlCmd
				if len(schema.Properties) == 0 {
					delete(g.schemas, typeName)
				}

				return false
			}
			return true
		})
	}
}

func (g *generator) generateYaml(version string) string {
	funcs := template.FuncMap{
		"includeOperation": func(operationId string) string {
			name := fmt.Sprintf("templates/operations/%s.yaml", operationId)
			b, err := resources.ReadFile(name)
			if err != nil {
				log.Fatalf("failed to include operation %s: %v", operationId, err)
			}
			return string(b)
		},
		"includeParameter": func(parameterName string) string {
			name := fmt.Sprintf("templates/parameters/%s.yaml", parameterName)
			b, err := resources.ReadFile(name)
			if err != nil {
				log.Fatalf("failed to include parameter %s: %v", parameterName, err)
			}
			return string(b)
		},
		"indent": func(i int, content string) string {
			indent := strings.Repeat(" ", i)

			var sb strings.Builder

			scanner := bufio.NewScanner(strings.NewReader(content))
			for scanner.Scan() {
				sb.WriteRune('\n')
				sb.WriteString(indent)
				sb.WriteString(scanner.Text())
			}

			return sb.String()
		},
		"stripPackage": func(typeName string) string {
			split := strings.SplitN(typeName, ".", 2)
			if len(split) > 1 {
				return split[1]
			} else {
				return typeName
			}
		},
	}

	t, err := template.New("openapi.yaml").Funcs(funcs).ParseFS(resources, "templates/openapi.yaml")
	if err != nil {
		log.Fatalf("failed to parse template: %v", err)
	}

	// get unique parameters
	parameters := make(map[string]bool)
	for _, methods := range g.paths {
		for _, operation := range methods {
			for i := range operation.Parameters {
				parameters[operation.Parameters[i]] = true
			}
		}
	}

	schemas := make(map[string]*Schema)
	for schemaName, schema := range g.schemas {
		for _, property := range schema.Properties {
			setPropertyType(property, property.typeName, g.schemas)
			setPropertyConstraint(property, property.validate)
		}

		// strip package names of schemas to have them globally sorted
		split := strings.SplitN(schemaName, ".", 2)
		if len(split) > 1 {
			schemas[split[1]] = schema
		} else {
			schemas[schemaName] = schema
		}

		// replace documentation links in enum description
		if len(schema.Enum) != 0 {
			var sb strings.Builder

			lines := strings.Split(schema.Description, "\n")

			v := 0
			prefix := "  - ["
			for _, line := range lines {
				if strings.HasPrefix(line, prefix) {
					a := len(prefix)
					b := strings.LastIndex(line, "]")

					sb.WriteString(line[:a-1])
					sb.WriteRune('`')
					sb.WriteString(schema.Enum[v])
					sb.WriteRune('`')
					sb.WriteString(line[b+1:])

					v++
				} else {
					sb.WriteString(line)
				}
				sb.WriteRune('\n')
			}

			if v != 0 && v != len(schema.Enum) {
				log.Fatalf("failed to replace documentation links in enum description: %v", schemaName)
			}

			schema.Description = sb.String()
		}
	}

	var yaml bytes.Buffer
	if err := t.Execute(&yaml, map[string]any{
		"parameters": parameters,
		"paths":      g.paths,
		"schemas":    schemas,
		"version":    version,
	}); err != nil {
		log.Fatalf("failed to execute OpenAPI template: %v", err)
	}
	return yaml.String()
}

func (g *generator) getPathConsts() map[string]string {
	f, err := parser.ParseFile(g.fileSet, g.sourcePath+"/http/common/common.go", nil, 0)
	if err != nil {
		log.Fatal(err)
	}

	var tokens []string
	ast.Inspect(f.Decls[0], func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.BasicLit:
			tokens = append(tokens, n.Value)
		case *ast.Ident:
			tokens = append(tokens, n.Name)
		}
		return true
	})

	pathConsts := make(map[string]string)
	for i := 0; i < len(tokens); i += 2 {
		name := tokens[i]
		if !strings.HasPrefix(name, "Path") {
			continue
		}
		pathConsts[name] = tokens[i+1][1 : len(tokens[i+1])-1]
	}

	return pathConsts
}

func describeEnum[V ~int](V) (string, []string) {
	var values []string
	for {
		i := len(values) + 1
		value := fmt.Sprintf("%v", V(i))
		if value == "" || (len(values) > 0 && value == values[len(values)-1]) {
			break // is empty or the default value
		}
		values = append(values, value)
	}
	return fmt.Sprintf("%T", V(0)), values
}

func extractType(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.ArrayType:
		return "[]" + extractType(expr.Elt)
	case *ast.Ident:
		return expr.Name
	case *ast.MapType:
		key := extractType(expr.Key)
		value := extractType(expr.Value)
		return fmt.Sprintf("%s:%s", key, value)
	case *ast.SelectorExpr:
		packageName := extractType(expr.X)
		typeName := extractType(expr.Sel)
		return fmt.Sprintf("%s.%s", packageName, typeName)
	case *ast.StarExpr:
		return extractType(expr.X)
	default:
		return ""
	}
}

func parseStructFieldTags(tags string) (json []string, validate []string) {
	if tags == "" {
		return
	}

	// e.g. `json:"retryCount,omitempty" validate:"gte=1"`
	for _, tag := range strings.Split(tags[1:len(tags)-1], " ") {
		// e.g. `json:"retryCount,omitempty"` -> [json,"retryCount,omitempty"]
		split := strings.Split(tag, ":")
		name, value := split[0], split[1]

		// e.g. "retryCount,omitempty" -> [retryCount,omitempty]
		values := strings.Split(value[1:len(value)-1], ",")

		switch name {
		case "json":
			if len(values) != 0 && values[0] != "-" { // if not skipped via "-"
				json = append(json, values...)
			}
		case "validate":
			validate = append(validate, values...)
		}
	}
	return
}

func setPropertyConstraint(property *Property, validate []string) {
	for _, tag := range validate {
		s := strings.SplitN(tag, "=", 2)

		// ignore subsequent parse errors
		// since the validate package parses those tag parameters too
		switch s[0] {
		case "gte":
			minimum, _ := strconv.ParseInt(s[1], 10, 32)
			property.Minimum = int(minimum)
		case "iso8601_duration":
			continue
		case "lte":
			maximum, _ := strconv.ParseInt(s[1], 10, 32)
			property.Maximum = int(maximum)
		case "max":
			if property.Type == "array" {
				maxItems, _ := strconv.ParseInt(s[1], 10, 32)
				property.MaxItems = int(maxItems)
			}
		case "min":
			if property.Type == "array" {
				minItems, _ := strconv.ParseInt(s[1], 10, 32)
				property.MinItems = int(minItems)
			}
		case "required":
			if !slices.Contains(validate, "dive") {
				property.required = true
			}
		case "unique":
			property.UniqueItems = true
		}
	}
}

func setPropertyType(property *Property, typeName string, schemas map[string]*Schema) {
	switch typeName {
	case "bool":
		property.Type = "boolean"
	case "int", "int32":
		property.Type = "integer"
		property.Format = "int32"
	case "int64":
		property.Type = "integer"
		property.Format = "int64"
	case "string":
		property.Type = "string"
	case "engine.ISO8601Duration":
		property.Type = "string"
	case "engine.Partition":
		property.Type = "string"
		property.Format = "date"
	case "time.Time":
		property.Type = "string"
		property.Format = "date-time"
	case "[]int32":
		property.Type = "array"
		property.Items = &Property{
			Type:   "integer",
			Format: "int32",
		}
	}

	if property.Type != "" {
		return
	}

	if strings.ContainsRune(typeName, ':') { // map type
		property.Type = "object"

		valueTypeName := strings.SplitN(typeName, ":", 2)[1]
		switch valueTypeName {
		case "string":
			property.AdditionalPropertiesType = "string"
			property.PropertyNamesPattern = server.RegexpTagName.String()
		default:
			property.AdditionalPropertiesTypeReference = valueTypeName
			property.PropertyNamesPattern = server.RegexpVariableName.String()
		}
		return
	}

	if strings.HasPrefix(typeName, "[]") {
		property.Type = "array"
		property.Items = &Property{}
		setPropertyType(property.Items, typeName[2:], schemas)
		return
	}

	schema, ok := schemas[typeName]
	if !ok {
		log.Fatalf("failed to look up type %s", typeName)
	}

	if schema.Type != "object" && len(schema.Enum) == 0 {
		setPropertyType(property, schema.Type, schemas)
	} else {
		property.TypeReference = typeName
	}

}

type Operation struct {
	Id         string
	Parameters []string
}

type Property struct {
	Name          string
	Description   string
	Format        string
	Type          string
	TypeReference string

	AdditionalPropertiesType          string
	AdditionalPropertiesTypeReference string
	PropertyNamesPattern              string

	Items *Property

	Maximum     int
	MaxItems    int
	Minimum     int
	MinItems    int
	UniqueItems bool

	required bool
	typeName string
	validate []string
}

type Schema struct {
	Description string
	Enum        []string
	Properties  []*Property
	Type        string
}

func (s *Schema) Required() []string {
	var required []string
	for _, p := range s.Properties {
		if p.required {
			required = append(required, p.Name)
		}
	}
	return required
}
