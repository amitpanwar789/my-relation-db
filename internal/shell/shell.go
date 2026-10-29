// This file connects statement input, SQL parsing, and displayed results.
// It repeatedly reads a statement and prints its parsed structure as JSON.
// It reports SQL errors and continues, while returning stream failures.
// Its input and output are replaceable streams so it can be tested easily.

// Package shell connects SQL input to AST inspection. It executes no SQL.
package shell

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/amitpanwar789/my-relation-db/internal/sql"
)

// Run reports statement errors and continues. Stream failures are returned to the
// caller. Interactive mode adds prompts; batch output contains only ASTs/errors.
func Run(input io.Reader, output io.Writer, interactive bool) error {
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
		stmt, parseErr := sql.Parse(text)
		if parseErr == nil && errors.Is(readErr, errIncompleteStatement) {
			tokens, _ := sql.Tokenize(text) // Parse succeeded, so tokenization cannot fail.
			parseErr = &sql.SyntaxError{Pos: tokens[len(tokens)-1].Pos, Message: "expected ; before end of input"}
		}
		if parseErr != nil {
			if _, err := fmt.Fprintf(output, "error: %v\n", parseErr); err != nil {
				return fmt.Errorf("write SQL error: %w", err)
			}
		} else {
			result := struct {
				Kind      string        `json:"kind"`
				Statement sql.Statement `json:"statement"`
			}{Kind: stmt.Kind(), Statement: stmt}
			if err := encoder.Encode(result); err != nil {
				return fmt.Errorf("write parsed statement: %w", err)
			}
		}
		if errors.Is(readErr, errIncompleteStatement) {
			return nil
		}
	}
}
