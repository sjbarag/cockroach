// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package schemaexpr_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/parser"
	"github.com/cockroachdb/cockroach/pkg/sql/schemaexpr"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/builtins"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
)

func TestIndexPredicateValidator_Validate(t *testing.T) {
	ctx := context.Background()
	semaCtx := tree.MakeSemaContext()

	// Trick to get the init() for the builtins package to run.
	_ = builtins.AllBuiltinNames

	database := tree.Name("foo")
	table := tree.Name("bar")
	tn := tree.MakeTableName(database, table)

	desc := testTableDesc(
		string(table),
		[]testCol{{"a", types.Bool}, {"b", types.Int}},
		[]testCol{{"c", types.String}},
	)

	validator := schemaexpr.MakeIndexPredicateValidator(ctx, tn, desc, &semaCtx)

	testData := []struct {
		expr          string
		expectedValid bool
		expectedExpr  string
	}{
		// Allow expressions that result in a bool.
		{"a", true, "a"},
		{"b = 0", true, "b = 0:::INT8"},
		{"a AND b = 0", true, "a AND (b = 0:::INT8)"},
		{"a IS NULL", true, "a IS NULL"},
		{"b IN (1, 2)", true, "b IN (1:::INT8, 2:::INT8)"},

		// Allow immutable functions.
		{"abs(b) > 0", true, "abs(b) > 0:::INT8"},
		{"c || c = 'foofoo'", true, "(c || c) = 'foofoo':::STRING"},
		{"lower(c) = 'bar'", true, "lower(c) = 'bar':::STRING"},

		// Disallow references to columns not in the table.
		{"d", false, ""},
		{"t.a", false, ""},

		// Disallow expressions that do not result in a bool.
		{"b", false, ""},
		{"abs(b)", false, ""},
		{"lower(c)", false, ""},

		// Disallow subqueries.
		{"exists(select 1)", false, ""},
		{"b IN (select 1)", false, ""},

		// Disallow mutable, aggregate, window, and set returning functions.
		{"b > random()", false, ""},
		{"sum(b) > 10", false, ""},
		{"row_number() OVER () > 1", false, ""},
		{"generate_series(1, 1) > 2", false, ""},

		// De-qualify column names.
		{"bar.a", true, "a"},
		{"foo.bar.a", true, "a"},
		{"bar.b = 0", true, "b = 0:::INT8"},
		{"foo.bar.b = 0", true, "b = 0:::INT8"},
		{"bar.a AND foo.bar.b = 0", true, "a AND (b = 0:::INT8)"},
	}

	for _, d := range testData {
		t.Run(d.expr, func(t *testing.T) {
			expr, err := parser.ParseExpr(d.expr)
			if err != nil {
				t.Fatalf("%s: unexpected error: %s", d.expr, err)
			}

			deqExpr, err := validator.Validate(expr)

			if !d.expectedValid {
				if err == nil {
					t.Fatalf("%s: expected invalid expression, but was valid", d.expr)
				}
				// The input expression is invalid so there is no need to check
				// the output expression r.
				return
			}

			if err != nil {
				t.Fatalf("%s: expected valid expression, but found error: %s", d.expr, err)
			}

			if deqExpr != d.expectedExpr {
				t.Errorf("%s: expected %q, got %q", d.expr, d.expectedExpr, deqExpr)
			}
		})
	}
}

func TestFormatIndexForDisplay(t *testing.T) {
	ctx := context.Background()
	semaCtx := tree.MakeSemaContext()

	database := tree.Name("foo")
	table := tree.Name("bar")
	tableName := tree.MakeTableName(database, table)

	colNames := []string{"a", "b"}
	tableDesc := testTableDesc(
		string(table),
		[]testCol{{colNames[0], types.Int}, {colNames[1], types.Int}},
		nil,
	)

	indexName := "baz"
	baseIndex := descpb.IndexDescriptor{
		Name:             indexName,
		ID:               0x0,
		ColumnNames:      colNames,
		ColumnDirections: []descpb.IndexDescriptor_Direction{descpb.IndexDescriptor_ASC, descpb.IndexDescriptor_DESC},
	}

	uniqueIndex := baseIndex
	uniqueIndex.Unique = true

	invertedIndex := baseIndex
	invertedIndex.Type = descpb.IndexDescriptor_INVERTED
	invertedIndex.ColumnNames = []string{"a"}

	storingIndex := baseIndex
	storingIndex.StoreColumnNames = []string{"c"}

	partialIndex := baseIndex
	partialIndex.Predicate = "a > 1:::INT8"

	testData := []struct {
		index      descpb.IndexDescriptor
		tableName  tree.TableName
		partition  string
		interleave string
		expected   string
	}{
		{baseIndex, descpb.AnonymousTable, "", "", "INDEX baz (a ASC, b DESC)"},
		{baseIndex, tableName, "", "", "INDEX baz ON foo.public.bar (a ASC, b DESC)"},
		{uniqueIndex, descpb.AnonymousTable, "", "", "UNIQUE INDEX baz (a ASC, b DESC)"},
		{invertedIndex, descpb.AnonymousTable, "", "", "INVERTED INDEX baz (a)"},
		{storingIndex, descpb.AnonymousTable, "", "", "INDEX baz (a ASC, b DESC) STORING (c)"},
		{partialIndex, descpb.AnonymousTable, "", "", "INDEX baz (a ASC, b DESC) WHERE a > 1:::INT8"},
		{
			partialIndex,
			descpb.AnonymousTable,
			" PARTITION BY LIST (a) (PARTITION p VALUES IN (2))",
			"",
			"INDEX baz (a ASC, b DESC) PARTITION BY LIST (a) (PARTITION p VALUES IN (2)) WHERE a > 1:::INT8",
		},
		{
			partialIndex,
			descpb.AnonymousTable,
			"",
			" INTERLEAVE IN PARENT par (a)",
			"INDEX baz (a ASC, b DESC) INTERLEAVE IN PARENT par (a) WHERE a > 1:::INT8",
		},
		{
			partialIndex,
			descpb.AnonymousTable,
			" PARTITION BY LIST (a) (PARTITION p VALUES IN (2))",
			" INTERLEAVE IN PARENT par (a)",
			"INDEX baz (a ASC, b DESC) INTERLEAVE IN PARENT par (a) PARTITION BY LIST (a) (PARTITION p VALUES IN (2)) WHERE a > 1:::INT8",
		},
	}

	for testIdx, tc := range testData {
		t.Run(strconv.Itoa(testIdx), func(t *testing.T) {
			got, err := schemaexpr.FormatIndexForDisplay(
				ctx, tableDesc, &tc.tableName, &tc.index, tc.partition, tc.interleave, &semaCtx,
			)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if got != tc.expected {
				t.Errorf("expected '%s', got '%s'", tc.expected, got)
			}
		})
	}
}
