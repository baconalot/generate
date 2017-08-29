package generate

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"errors"

	"io"

	"github.com/baconalot/generate/jsonschema"
)

// Generator will produce structs from the JSON schema.
type Generator struct {
	schema               *jsonschema.Schema
	nonRequiredAsPointer bool
}

// New creates an instance of a generator which will produce structs.
func New(schema *jsonschema.Schema) *Generator {
	return &Generator{
		schema:               schema,
		nonRequiredAsPointer: false,
	}
}

func (g *Generator) Generate(ppackagee string, w io.Writer) (err error) {
	types, err := g.CreateStructs()
	if err != nil {
		return err
	}

	//header
	fmt.Fprintf(w, "package %v\n\n", ppackagee)
	fmt.Fprintf(w, "import \"encoding/json\"\n\n")

	for _, s := range types {
		switch s.Type {
		case GTypeStruct:
			g.writeStruct(s, w)
		case GTypeUndefinedStruct:
			g.writeSingleType(s, "map[string]interface{}", w)
		case GTypeFloat:
			g.writeSingleType(s, "json.Number", w)
		case GTypeInt:
			g.writeSingleType(s, "int", w)
		case GTypeString:
			g.writeSingleType(s, "string", w)
		}
	}
	return nil
}

func (g *Generator) writeStruct(s GoType, w io.Writer) {
	fmt.Fprintf(w, "type %s struct {\n", s.Name)
	for _, f := range s.Fields {
		// Only apply omitempty if the field is not required.
		omitempty := ",omitempty"
		if f.Required {
			omitempty = ""
		}
		jsontag := ""
		if f.JSONName != "" {
			jsontag = fmt.Sprintf("`json:\"%s%s\"`", f.JSONName, omitempty)
		}

		fmt.Fprintf(w, "	%v %v %v\n", f.Name, f.Type, jsontag)
	}
	fmt.Fprintf(w, "}\n\n")
}

func (g *Generator) writeSingleType(s GoType, ty string, w io.Writer) {
	fmt.Fprintf(w, "type %v %v\n\n", s.Name, ty)
}

// CreateStructs creates structs from the JSON schema, keyed by the golang name.
func (g *Generator) CreateStructs() (structs map[string]GoType, err error) {
	structs = make(map[string]GoType)

	// Extract nested and complex types from the JSON schema.
	types := g.schema.ExtractTypes()

	errs := []error{}

	for _, typeKey := range getOrderedKeyNamesFromSchemaMap(types) {
		v := types[typeKey]
		if strings.Contains(typeKey, "properties/") && v.Type != "object" { //arg
			continue
		}

		var gtyp GType
		var fields map[string]Field
		var err error

		switch v.Type {
		case "object", "array":
			gtyp = GTypeStruct
			fields, err = g.getFields(typeKey, v.Properties, types, v.Required)
			if len(fields) <= 0 {
				gtyp = GTypeUndefinedStruct
			}
		case "integer": //type foo struct{int64}
			gtyp = GTypeInt
			fields = map[string]Field{"": Field{Type: "int"}}
		case "string": //type foo struct{string}
			gtyp = GTypeString
			fields = map[string]Field{"": Field{Type: "string"}}
		default:
			err = fmt.Errorf("Unknown type for output: %v", v.Type)
		}
		if err != nil {
			errs = append(errs, err)
		}

		structName := getStructName(typeKey, v, 1)
		if err != nil {
			errs = append(errs, err)
		}

		s := GoType{
			ID:     typeKey,
			Name:   structName,
			Fields: fields,
			Type:   gtyp,
		}

		structs[s.Name] = s
	}

	if len(errs) > 0 {
		return structs, errors.New(joinErrors(errs))
	}

	return structs, nil
}

func joinErrors(errs []error) string {
	var buffer bytes.Buffer

	for idx, err := range errs {
		buffer.WriteString(err.Error())

		if idx+1 < len(errs) {
			buffer.WriteString(", ")
		}
	}

	return buffer.String()
}

func getOrderedKeyNamesFromSchemaMap(m map[string]*jsonschema.Schema) []string {
	keys := make([]string, len(m))
	idx := 0
	for k := range m {
		keys[idx] = k
		idx++
	}
	sort.Strings(keys)
	return keys
}

func (g *Generator) getFields(parentTypeKey string, properties map[string]*jsonschema.Schema, types map[string]*jsonschema.Schema, requiredFields []string) (field map[string]Field, err error) {
	fields := map[string]Field{}

	missingTypes := []string{}
	errors := []error{}

	for _, fieldName := range getOrderedKeyNamesFromSchemaMap(properties) {
		v := properties[fieldName]
		required := contains(requiredFields, fieldName)

		golangName := getGolangName(fieldName)
		tn, err := getTypeForField(parentTypeKey, fieldName, golangName, v, types, !required && g.nonRequiredAsPointer)

		if err != nil {
			missingTypes = append(missingTypes, golangName)
			errors = append(errors, err)
		}

		f := Field{
			Name:     golangName,
			JSONName: fieldName,
			// Look up the types, try references first, then drop to the built-in types.
			Type:     tn,
			Required: required,
		}

		fields[f.Name] = f
	}

	if len(missingTypes) > 0 {
		return fields, fmt.Errorf("missing types for '%s' with errors %s\n", strings.Join(missingTypes, ", "), joinErrors(errors))
	}

	return fields, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func getTypeForField(parentTypeKey string, fieldName string, fieldGoName string, fieldSchema *jsonschema.Schema, types map[string]*jsonschema.Schema, pointer bool) (typeName string, err error) {
	if fieldSchema == nil {
		return "interface{}", nil
	}

	majorType := fieldSchema.Type
	subType := ""

	// Look up by named reference.
	if fieldSchema.Reference != "" {
		if t, ok := types[fieldSchema.Reference]; ok {
			sn := getStructName(fieldSchema.Reference, t, 1)

			majorType = "object"
			subType = sn
		}
	}

	// Look up any embedded types.
	if subType == "" && majorType == "object" {
		if parentType, ok := types[parentTypeKey+"/properties/"+fieldName]; ok {
			sn := getStructName(parentTypeKey+"/properties/"+fieldName, parentType, 1)

			majorType = "object"
			subType = sn
		}
	}

	// Find named array references.
	if majorType == "array" {
		s, _ := getTypeForField(parentTypeKey, fieldName, fieldGoName, fieldSchema.Items, types, false)
		subType = s
	}

	name, err := getPrimitiveTypeName(majorType, subType, pointer)

	if err != nil {
		return name, fmt.Errorf("Failed to get the type for %v, majorType %v, subType %v, with error %v\n",
			fieldGoName,
			majorType,
			subType,
			err)
	}

	return name, nil
}

func getPrimitiveTypeName(schemaType string, subType string, pointer bool) (name string, err error) {
	switch schemaType {
	case "array":
		if subType == "" {
			return "error_creating_array", errors.New("can't create an array of an empty subtype")
		}
		return "[]" + subType, nil
	case "boolean":
		return "bool", nil
	case "integer":
		return "int", nil
	case "number":
		return "json.Number", nil
	case "null":
		return "nil", nil
	case "object":
		if pointer {
			return "*" + subType, nil
		}

		return subType, nil
	case "string":
		return "string", nil
	}

	return "undefined", fmt.Errorf("failed to get a primitive type for schemaType %s and subtype %s\n", schemaType, subType)
}

// getStructName makes a golang struct name from an input reference in the form of #/definitions/address
// The parts refers to the number of segments from the end to take as the name.
func getStructName(reference string, structType *jsonschema.Schema, n int) string {
	if reference == "#" {
		rootName := structType.Title

		if rootName == "" {
			rootName = structType.Description
		}

		if rootName == "" {
			rootName = "Root"
		}

		return getGolangName(rootName)
	}

	clean := strings.Replace(reference, "#/", "", -1)
	parts := strings.Split(clean, "/")
	partsToUse := parts[len(parts)-n:]

	sb := bytes.Buffer{}

	for _, p := range partsToUse {
		sb.WriteString(getGolangName(p))
	}

	result := sb.String()

	if result == "" {
		return "Root"
	}

	return result
}

// getGolangName strips invalid characters out of golang struct or field names.
func getGolangName(s string) string {
	buf := bytes.NewBuffer([]byte{})

	for _, v := range splitOnAll(s, '_', ' ', '.', '-') {
		buf.WriteString(capitaliseFirstLetter(v))
	}

	return buf.String()
}

func splitOnAll(s string, splitItems ...rune) []string {
	rv := []string{}

	buf := bytes.NewBuffer([]byte{})
	for _, c := range s {
		if matches(c, splitItems) {
			rv = append(rv, buf.String())
			buf.Reset()
		} else {
			buf.WriteRune(c)
		}
	}
	if buf.Len() > 0 {
		rv = append(rv, buf.String())
	}

	return rv
}

func matches(c rune, any []rune) bool {
	for _, a := range any {
		if a == c {
			return true
		}
	}
	return false
}

func capitaliseFirstLetter(s string) string {
	if s == "" {
		return s
	}

	prefix := s[0:1]
	suffix := s[1:]
	return strings.ToUpper(prefix) + suffix
}

type GType int

const (
	GTypeNOTSET GType = iota
	GTypeStruct
	GTypeUndefinedStruct
	GTypeInt
	GTypeString
	GTypeFloat
)

// Struct defines the data required to generate a struct in Go.
type GoType struct {
	// The ID within the JSON schema, e.g. #/definitions/address
	ID string
	// The golang name, e.g. "Address"
	Name   string
	Fields map[string]Field
	Type   GType
}

// Field defines the data required to generate a field in Go.
type Field struct {
	// The golang name, e.g. "Address1"
	Name string
	// The JSON name, e.g. "address1"
	JSONName string
	// The golang type of the field, e.g. a built-in type like "string" or the name of a struct generated from the JSON schema.
	Type string
	// Required is set to true when the field is required.
	Required bool
}
