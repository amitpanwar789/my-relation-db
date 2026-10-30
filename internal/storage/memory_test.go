// This file tests the row store without SQL, schemas, or execution plans.
// It appends typed rows and checks snapshots, empty tables, and isolated stores.
// Mutations of inputs and scan results must never change previously stored rows.
// NULL, empty text, zero, and false remain separate values throughout storage.

package storage_test

import (
	"reflect"
	"testing"

	"github.com/amitpanwar789/my-relation-db/internal/storage"
	"github.com/amitpanwar789/my-relation-db/internal/types"
)

func TestMemorySnapshotsAndIsolation(t *testing.T) {
	var store storage.Memory
	empty, err := store.Scan("t")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty scan: %v, %v", empty, err)
	}
	row := []types.Value{types.IntegerValue(0), types.TextValue(""), types.BooleanValue(false), types.NullValue(types.Text)}
	want := append([]types.Value(nil), row...)
	if err := store.Append("t", row); err != nil {
		t.Fatal(err)
	}
	row[0] = types.IntegerValue(9)
	first, err := store.Scan("t")
	if err != nil || !reflect.DeepEqual(first, [][]types.Value{want}) {
		t.Fatalf("stored row: %v, %v", first, err)
	}
	first[0][1] = types.TextValue("external")
	first[0] = nil
	if err := store.Append("t", want); err != nil {
		t.Fatal(err)
	}
	if err := store.Append("other", []types.Value{types.IntegerValue(8)}); err != nil {
		t.Fatal(err)
	}
	fresh, err := store.Scan("t")
	if err != nil || !reflect.DeepEqual(fresh, [][]types.Value{want, want}) {
		t.Fatalf("snapshot changed storage: %v, %v", fresh, err)
	}
	var separate storage.Memory
	empty, err = separate.Scan("t")
	if err != nil || len(empty) != 0 {
		t.Fatal("independent store inherited data")
	}
}

func TestMemoryReplacementOwnership(t *testing.T) {
	var store storage.Memory
	rows := [][]types.Value{{types.IntegerValue(1)}, {types.IntegerValue(1)}}
	if err := store.Replace("t", rows); err != nil {
		t.Fatal(err)
	}
	rows[0][0] = types.IntegerValue(9)
	rows[1] = nil
	got, err := store.Scan("t")
	if err != nil || !reflect.DeepEqual(got, [][]types.Value{{types.IntegerValue(1)}, {types.IntegerValue(1)}}) {
		t.Fatalf("replacement shares inputs: %v, %v", got, err)
	}
	if err := store.Replace("other", [][]types.Value{{types.IntegerValue(2)}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Replace("t", nil); err != nil {
		t.Fatal(err)
	}
	empty, err := store.Scan("t")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatal("replacement failed to clear table")
	}
	other, err := store.Scan("other")
	if err != nil || other[0][0] != types.IntegerValue(2) {
		t.Fatal("replacement changed another table")
	}
}
