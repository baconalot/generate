package main

import (
	"bytes"
	"flag"
	"io"
	"io/ioutil"
	"reflect"
	"strings"
	"testing"

	"github.com/a-h/generate"
)

func TestThatFieldNamesAreOrdered(t *testing.T) {
	m := map[string]generate.Field{
		"z": generate.Field{},
		"b": generate.Field{},
	}

	actual := getOrderedFieldNames(m)
	expected := []string{"b", "z"}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("expected %s and actual %s should match in order", strings.Join(expected, ", "), strings.Join(actual, ","))
	}
}

func TestThatStructNamesAreOrdered(t *testing.T) {
	m := map[string]generate.Struct{
		"c": generate.Struct{},
		"b": generate.Struct{},
		"a": generate.Struct{},
	}

	actual := getOrderedStructNames(m)
	expected := []string{"a", "b", "c"}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("expected %s and actual %s should match in order", strings.Join(expected, ", "), strings.Join(actual, ","))
	}
}

func TestThatThePackageCanBeSet(t *testing.T) {
	pkg := "testpackage"
	p = &pkg

	r, w := io.Pipe()

	go output(w, make(map[string]generate.Struct))

	lr := io.LimitedReader{R: r, N: 20}
	bs, _ := ioutil.ReadAll(&lr)
	output := bytes.NewBuffer(bs).String()

	if output != "package testpackage\n" {
		t.Error("Unexpected package declaration: ", output)
	}
}

func TestLineAndCharacterFromOffset(t *testing.T) {
	tests := []struct {
		In                []byte
		Offset            int
		ExpectedLine      int
		ExpectedCharacter int
		ExpectedError     bool
	}{
		{
			In:                []byte("Line 1\nLine 2"),
			Offset:            6,
			ExpectedLine:      2,
			ExpectedCharacter: 1,
		},
		{
			In:                []byte("Line 1\r\nLine 2"),
			Offset:            7,
			ExpectedLine:      2,
			ExpectedCharacter: 1,
		},
		{
			In:                []byte("Line 1\nLine 2"),
			Offset:            0,
			ExpectedLine:      1,
			ExpectedCharacter: 1,
		},
		{
			In:                []byte("Line 1\nLine 2"),
			Offset:            200,
			ExpectedLine:      0,
			ExpectedCharacter: 0,
			ExpectedError:     true,
		},
		{
			In:                []byte("Line 1\nLine 2"),
			Offset:            -1,
			ExpectedLine:      0,
			ExpectedCharacter: 0,
			ExpectedError:     true,
		},
	}

	for _, test := range tests {
		actualLine, actualCharacter, err := lineAndCharacter(test.In, test.Offset)
		if err != nil && !test.ExpectedError {
			t.Errorf("Unexpected error for input %s at offset %d: %v", test.In, test.Offset, err)
			continue
		}

		if actualLine != test.ExpectedLine || actualCharacter != test.ExpectedCharacter {
			t.Errorf("For '%s' at offset %d, expected %d:%d, but got %d:%d", test.In, test.Offset, test.ExpectedLine, test.ExpectedCharacter, actualLine, actualCharacter)
		}
	}
}

func TestMainReally(t *testing.T) {
	// flag.Set("i", "../testfiles/min_err.json")
	flag.Set("i", "../testfiles/ortb23req.json")
	main()
}
