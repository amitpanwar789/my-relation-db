// This file connects SQL input to a session and displays its results.
// It parses complete statements before checking their meaning against schemas.
// It reports SQL errors and continues, while returning stream failures.
// Its input and output are replaceable streams so it can be tested easily.

// Package shell connects SQL streams to session metadata and validation.
package shell

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/amitpanwar789/my-relation-db/internal/engine"
	"github.com/amitpanwar789/my-relation-db/internal/sql"
)

// Run reports statement errors and continues. Stream failures are returned to the
// caller. Interactive mode adds prompts; batch output contains results/errors.
func Run(input io.Reader, output io.Writer, interactive bool) error {
	session := &engine.Session{}
	source := statementInput{reader: bufio.NewReader(input)}
	if interactive {
		source.newline = func() error {
			_, err := io.WriteString(output, " -> ")
			return err
		}
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	for {
		if interactive {
			if _, err := io.WriteString(output, "db> "); err != nil {
				return fmt.Errorf("write prompt: %w", err)
			}
		}
		text, readErr := source.next()
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil && !errors.Is(readErr, errIncompleteStatement) {
			return fmt.Errorf("collect SQL input: %w", readErr)
		}
		if strings.TrimSpace(text) == ";" {
			continue
		}
		stmt, statementErr := sql.Parse(text)
		if statementErr == nil && errors.Is(readErr, errIncompleteStatement) {
			tokens, _ := sql.Tokenize(text) // Parse succeeded, so tokenization cannot fail.
			statementErr = &sql.SyntaxError{Pos: tokens[len(tokens)-1].Pos, Message: "expected ; before end of input"}
		}
		var result engine.Result
		if statementErr == nil {
			result, statementErr = session.Apply(stmt)
		}
		if statementErr != nil {
			if _, err := fmt.Fprintf(output, "error: %v\n", statementErr); err != nil {
				return fmt.Errorf("write SQL error: %w", err)
			}
		} else {
			if err := encoder.Encode(result); err != nil {
				return fmt.Errorf("write session result: %w", err)
			}
		}
		if errors.Is(readErr, errIncompleteStatement) {
			return nil
		}
	}
}
