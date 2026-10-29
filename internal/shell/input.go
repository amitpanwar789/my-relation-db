// This file collects one complete SQL statement from an input stream.
// It stops at a semicolon outside a quoted string and preserves the text.
// It supports multiline statements and requests continuation prompts.
// Unfinished input and stream failures are passed back to the shell.

package shell

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

// statementInput frames input without assigning SQL meaning to its contents.
type statementInput struct {
	reader  *bufio.Reader
	newline func() error
}

var errIncompleteStatement = errors.New("SQL statement missing its terminating semicolon")

func (s *statementInput) next() (string, error) {
	var text strings.Builder
	inString := false
	for {
		b, err := s.reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) && strings.TrimSpace(text.String()) != "" {
				return text.String(), errIncompleteStatement
			}
			return text.String(), err
		}
		text.WriteByte(b)
		if b == '\'' {
			// Doubled quotes toggle twice, with no delimiter between them.
			inString = !inString
		}
		if b == ';' && !inString {
			return text.String(), nil
		}
		if b == '\n' && strings.TrimSpace(text.String()) != "" && s.newline != nil {
			if err := s.newline(); err != nil {
				return "", err
			}
		}
	}
}
