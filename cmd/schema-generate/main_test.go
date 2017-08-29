package main

import (
	"flag"
	"testing"
)

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
	flag.Set("i", "../testfiles/min_err.json")
	// flag.Set("i", "../testfiles/ortb23req.json")
	main()
}
