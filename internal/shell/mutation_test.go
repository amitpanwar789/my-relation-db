// This file checks Stage 4 SQL through the shell's real input/output boundary.
// It decodes results to verify filtered rows and explicit zero mutation counts.
// Errors and incomplete statements must not become successful mutations.
// Existing stream framing and session behavior remain shared with earlier stages.

package shell_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/shell"
)

func TestShellFilteringAndMutationResults(t *testing.T) {
	input := `CREATE TABLE t (id INTEGER, name TEXT, active BOOLEAN);
 INSERT INTO t (id, name, active) VALUES (1, 'Amit', TRUE);
 INSERT INTO t (id, name) VALUES (2, 'Neha');
 SELECT name FROM t WHERE id = 1;
 UPDATE t SET name = 'Changed', active = FALSE WHERE active IS NULL;
 SELECT name FROM t WHERE active = FALSE;
 DELETE FROM t WHERE id = 99;
 UPDATE t SET active = TRUE WHERE FALSE;
 DELETE FROM t;
 SELECT * FROM t WHERE TRUE;`
	var output bytes.Buffer
	if err := shell.Run(strings.NewReader(input), &output, false); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	results := make([]result, 10)
	for i := range results {
		if err := decoder.Decode(&results[i]); err != nil {
			t.Fatal(err)
		}
	}
	if results[3].Query.Columns[0].Name != "name" || string(results[3].Query.Rows[0][0].Value) != `"Amit"` {
		t.Fatal("filter-before-project failed")
	}
	if results[4].Status != "updated" || results[4].AffectedRows != 1 || results[4].Query != nil {
		t.Fatal("update acknowledgement incorrect")
	}
	if string(results[5].Query.Rows[0][0].Value) != `"Changed"` {
		t.Fatal("updated row not visible")
	}
	if results[6].Status != "deleted" || results[6].AffectedRows != 0 || results[7].Status != "updated" || results[7].AffectedRows != 0 || results[8].AffectedRows != 2 {
		t.Fatal("mutation counts incorrect")
	}
	if results[9].Query.Rows == nil || len(results[9].Query.Rows) != 0 || len(results[9].Query.Columns) != 3 {
		t.Fatal("empty result lost metadata")
	}
	// Decoding into an int cannot distinguish a missing field from zero.
	output.Reset()
	if err := shell.Run(strings.NewReader("CREATE TABLE t (id INTEGER); UPDATE t SET id = 1; DELETE FROM t;"), &output, false); err != nil {
		t.Fatal(err)
	}
	if strings.Count(output.String(), `"affected_rows": 0`) != 2 {
		t.Fatalf("zero count omitted: %s", output.String())
	}
}

func TestShellMutationRecovery(t *testing.T) {
	for _, incomplete := range []string{"UPDATE t SET id = 9", "DELETE FROM t"} {
		var output bytes.Buffer
		input := "CREATE TABLE t (id INTEGER); INSERT INTO t (id) VALUES (1); UPDATE t SET id = 'wrong'; DELETE FROM t WHERE TRUE OR missing = 1; SELECT * FROM t; " + incomplete
		if err := shell.Run(strings.NewReader(input), &output, false); err != nil {
			t.Fatal(err)
		}
		text := output.String()
		if strings.Count(text, "semantic error:") != 2 || !strings.Contains(text, "expected ; before end of input") || strings.Contains(text, `"status": "updated"`) || strings.Contains(text, `"status": "deleted"`) || !strings.Contains(text, `"value": 1`) {
			t.Fatalf("mutation recovery failed: %s", text)
		}
	}
}
