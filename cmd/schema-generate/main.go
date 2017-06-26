package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"

	"encoding/json"

	"github.com/a-h/generate"
	"github.com/a-h/generate/jsonschema"
)

var (
	i = flag.String("i", "", "The input JSON Schema file.")
	o = flag.String("o", "", "The output file for the schema.")
	p = flag.String("p", "main", "The package that the structs are created in.")
)

func main() {
	flag.Parse()

	b, err := ioutil.ReadFile(*i)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to read the input file with error ", err)
		return
	}

	schema, err := jsonschema.Parse(string(b))

	if err != nil {
		if jsonError, ok := err.(*json.SyntaxError); ok {
			line, character, lcErr := lineAndCharacter(b, int(jsonError.Offset))
			fmt.Fprintf(os.Stderr, "Cannot parse JSON schema due to a syntax error at line %d, character %d: %v\n", line, character, jsonError.Error())
			if lcErr != nil {
				fmt.Fprintf(os.Stderr, "Couldn't find the line and character position of the error due to error %v\n", lcErr)
			}
			return
		}
		if jsonError, ok := err.(*json.UnmarshalTypeError); ok {
			line, character, lcErr := lineAndCharacter(b, int(jsonError.Offset))
			fmt.Fprintf(os.Stderr, "The JSON type '%v' cannot be converted into the Go '%v' type on struct '%s', field '%v'. See input file line %d, character %d\n", jsonError.Value, jsonError.Type.Name(), jsonError.Struct, jsonError.Field, line, character)
			if lcErr != nil {
				fmt.Fprintf(os.Stderr, "Couldn't find the line and character position of the error due to error %v\n", lcErr)
			}
			return
		}
		fmt.Fprintln(os.Stderr, "Failed to parse the input JSON schema with error ", err)
		return
	}

	g := generate.New(schema)

	structs, err := g.CreateStructs()

	if err != nil {
		fmt.Fprintln(os.Stderr, "Failure generating structs: ", err)
	}

	var w io.Writer

	if *o == "" {
		w = os.Stdout
	} else {
		w, err = os.Create(*o)

		if err != nil {
			fmt.Fprintln(os.Stderr, "Error opening output file: ", err)
			return
		}
	}

	output(w, structs)
}

func lineAndCharacter(bytes []byte, offset int) (line int, character int, err error) {
	lf := byte(0x0A)

	if offset > len(bytes) {
		return 0, 0, fmt.Errorf("Couldn't find offset %d in bytes.", offset)
	}

	// Humans tend to count from 1.
	line = 1

	for i, b := range bytes {
		if b == lf {
			line++
			character = 0
		}
		character++
		if i == offset {
			return line, character, nil
		}
	}

	return 0, 0, fmt.Errorf("Couldn't find offset %d in bytes.", offset)
}

func getOrderedFieldNames(m map[string]generate.Field) []string {
	keys := make([]string, len(m))
	idx := 0
	for k := range m {
		keys[idx] = k
		idx++
	}
	sort.Strings(keys)
	return keys
}

func getOrderedStructNames(m map[string]generate.Struct) []string {
	keys := make([]string, len(m))
	idx := 0
	for k := range m {
		keys[idx] = k
		idx++
	}
	sort.Strings(keys)
	return keys
}

func output(w io.Writer, structs map[string]generate.Struct) {
	//TODO: Use templates.
	fmt.Fprintf(w, "package %v\n", *p)

	for _, k := range getOrderedStructNames(structs) {
		s := structs[k]

		fmt.Fprintln(w, "")
		fmt.Fprintf(w, "type %s struct {\n", s.Name)

		for _, fieldKey := range getOrderedFieldNames(s.Fields) {
			f := s.Fields[fieldKey]

			// Only apply omitempty if the field is not required.
			omitempty := ",omitempty"
			if f.Required {
				omitempty = ""
			}
			jsontag := ""
			if f.JSONName != "" {
				jsontag = fmt.Sprintf("`json:\"%s%s\"`", f.JSONName, omitempty)
			}

			fmt.Fprintf(w, "  %v %v %v\n", f.Name, f.Type, jsontag)
		}

		fmt.Fprintln(w, "}")
	}
}
