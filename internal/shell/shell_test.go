// This file tests the complete input-to-output path of the SQL shell.
// It supplies text streams and checks session results, prompts, and errors.
// It covers multiline input, quoted semicolons, recovery, and unfinished SQL.
// Failing streams verify that input and output errors reach the caller.

package shell_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/catalog"
	"github.com/amitpanwar789/my-relation-db/internal/shell"
)

type result struct {
	Kind    string          `json:"kind"`
	Status  string          `json:"status"`
	Command json.RawMessage `json:"command"`
}

func TestSQLSession(t *testing.T) {
	input := `
CREATE TABLE Users (
  id INTEGER,
  name TEXT,
  active BOOLEAN
);
INSERT INTO Users (id, name, active) VALUES (9223372036854775807, 'O''Brien; नमस्ते', TRUE);
SELECT id, name FROM Users; SELECT * FROM users;
; ;
`
	var output bytes.Buffer
	if err := shell.Run(strings.NewReader(input), &output, false); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	var results []result
	for {
		var r result
		err := decoder.Decode(&r)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		results = append(results, r)
	}
	wantKinds := []string{"create_table", "insert", "select", "select"}
	if len(results) != len(wantKinds) {
		t.Fatalf("got %d results, want %d", len(results), len(wantKinds))
	}
	for i, kind := range wantKinds {
		if results[i].Kind != kind {
			t.Errorf("result %d kind = %q; want %q", i, results[i].Kind, kind)
		}
	}
	if results[0].Status != "created-in-memory" {
		t.Fatalf("CREATE did not report registration: %+v", results[0])
	}
	for _, result := range results[1:] {
		if result.Status != "validated-only" {
			t.Fatalf("row operation claimed execution: %+v", result)
		}
	}
	var inserted struct {
		Row []struct {
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
		} `json:"prepared_row"`
	}
	if err := json.Unmarshal(results[1].Command, &inserted); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := json.Unmarshal(inserted.Row[1].Value, &name); err != nil {
		t.Fatal(err)
	}
	if string(inserted.Row[0].Value) != "9223372036854775807" || name != "O'Brien; नमस्ते" {
		t.Fatalf("typed contents changed: %+v", inserted.Row)
	}
	var selected struct {
		Columns   []catalog.Column `json:"columns"`
		Positions []int            `json:"positions"`
	}
	if err := json.Unmarshal(results[2].Command, &selected); err != nil {
		t.Fatal(err)
	}
	if len(selected.Columns) != 2 || selected.Columns[1].Name != "name" || selected.Positions[1] != 1 {
		t.Fatalf("unexpected bound projection: %+v", selected)
	}
}

func TestRecoveryAfterRejectedStatement(t *testing.T) {
	var output bytes.Buffer
	input := "CREATE TABLE t (a TEXT); SELECT FROM t; SELECT @ FROM t; SELECT * FROM t WHERE a; SELECT * FROM t;"
	if err := shell.Run(strings.NewReader(input), &output, false); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Count(text, "error:") != 3 || strings.Count(text, `"kind": "select"`) != 1 {
		t.Fatalf("expected three errors followed by one AST, got:\n%s", text)
	}
	if !strings.Contains(text, "line 1, column 9") || !strings.Contains(text, "unexpected character") {
		t.Fatalf("missing positioned diagnostics: %s", text)
	}
}

func TestIncompleteInput(t *testing.T) {
	for _, tt := range []struct{ input, message string }{
		{"SELECT * FROM t", "expected ; before end of input"},
		{"SELECT", "expected a column name or *"},
		{"INSERT INTO t (a) VALUES ('never ends; SELECT * FROM t;", "unterminated string literal"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			var output bytes.Buffer
			if err := shell.Run(strings.NewReader(tt.input), &output, false); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), tt.message) || strings.Contains(output.String(), `"kind"`) {
				t.Fatalf("unexpected EOF result: %s", output.String())
			}
		})
	}
}

func TestQuoteFraming(t *testing.T) {
	for _, value := range []string{"''", "''''", "'a;'';b'", "'a\nb'", "'\\'", "'é;🙂'"} {
		t.Run(value, func(t *testing.T) {
			var output bytes.Buffer
			input := "CREATE TABLE t (a TEXT); INSERT INTO t (a) VALUES (" + value + ");SELECT * FROM t;"
			if err := shell.Run(strings.NewReader(input), &output, false); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(output.String(), "error:") || !strings.Contains(output.String(), `"kind": "insert"`) || !strings.Contains(output.String(), `"kind": "select"`) {
				t.Fatalf("bad quoted-string framing: %s", output.String())
			}
		})
	}
}

func TestEmptyInputAndStatements(t *testing.T) {
	for _, input := range []string{"", " \t\n", ";", " ;\n; ; \n"} {
		var output bytes.Buffer
		if err := shell.Run(strings.NewReader(input), &output, false); err != nil || output.Len() != 0 {
			t.Errorf("input %q: output %q, error %v", input, output.String(), err)
		}
	}
}

func TestInteractivePrompts(t *testing.T) {
	var output bytes.Buffer
	if err := shell.Run(strings.NewReader("CREATE TABLE t (\nname TEXT);\n"), &output, true); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output.String(), "db>  -> ") || !strings.HasSuffix(output.String(), "db> ") {
		t.Fatalf("unexpected prompts: %q", output.String())
	}
}

var streamFailure = errors.New("test stream failure")

type brokenReader struct{ err error }

func (r brokenReader) Read([]byte) (int, error) { return 0, r.err }

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, streamFailure }

func TestStreamFailures(t *testing.T) {
	for _, tt := range []struct {
		name        string
		input       io.Reader
		output      io.Writer
		interactive bool
		want        error
	}{
		{"input", brokenReader{streamFailure}, io.Discard, false, streamFailure},
		{"unexpected EOF from device", brokenReader{io.ErrUnexpectedEOF}, io.Discard, false, io.ErrUnexpectedEOF},
		{"result output", strings.NewReader("CREATE TABLE t (a TEXT);"), brokenWriter{}, false, streamFailure},
		{"error output", strings.NewReader("SELECT;"), brokenWriter{}, false, streamFailure},
		{"prompt output", strings.NewReader(""), brokenWriter{}, true, streamFailure},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := shell.Run(tt.input, tt.output, tt.interactive)
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want wrapped %v", err, tt.want)
			}
		})
	}
}

type promptThenFailWriter struct{ writes int }

func (w *promptThenFailWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > 1 {
		return 0, streamFailure
	}
	return len(p), nil
}

func TestContinuationPromptFailure(t *testing.T) {
	err := shell.Run(strings.NewReader("SELECT\n"), &promptThenFailWriter{}, true)
	if !errors.Is(err, streamFailure) {
		t.Fatalf("continuation output failure was swallowed: %v", err)
	}
}

func TestSemanticRecoveryAndSessionLifetime(t *testing.T) {
	var output bytes.Buffer
	input := `CREATE TABLE t (a INTEGER, a TEXT);
CREATE TABLE t (a INTEGER, name TEXT, active BOOLEAN);
INSERT INTO t (a) VALUES ('wrong');
SELECT missing FROM t;
CREATE TABLE t (other BOOLEAN);
INSERT INTO t (name, a) VALUES ('Amit', 42);
SELECT a FROM t;`
	if err := shell.Run(strings.NewReader(input), &output, false); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Count(text, "semantic error:") != 4 || strings.Count(text, "created-in-memory") != 1 || strings.Count(text, "validated-only") != 2 {
		t.Fatalf("unexpected semantic recovery: %s", text)
	}
	if !strings.Contains(text, `"value": null`) || strings.Contains(text, `"rows"`) {
		t.Fatalf("missing NULL or fabricated row output: %s", text)
	}
	output.Reset()
	if err := shell.Run(strings.NewReader("SELECT a FROM t;"), &output, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "unknown table") {
		t.Fatal("new shell run retained previous session state")
	}
}

func TestIncompleteCreateDoesNotApply(t *testing.T) {
	var output bytes.Buffer
	if err := shell.Run(strings.NewReader("CREATE TABLE t (a TEXT)"), &output, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "created-in-memory") || !strings.Contains(output.String(), "expected ;") {
		t.Fatalf("incomplete CREATE was applied: %s", output.String())
	}
}
