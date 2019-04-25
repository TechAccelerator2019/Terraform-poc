package lang

import (
	"testing"

	"github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/hcl2/hcl/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// TestFunctions tests that functions are callable through the functionality
// in the langs package, via HCL.
//
// These tests are primarily here to assert that the functions are properly
// registered in the functions table, rather than to test all of the details
// of the functions. Each function should only have one or two tests here,
// since the main set of unit tests for a function should live alongside that
// function either in the "funcs" subdirectory here or over in the cty
// function/stdlib package.
//
// One exception to that is we can use this test mechanism to assert common
// patterns that are used in real-world configurations which rely on behaviors
// implemented either in this lang package or in HCL itself, such as automatic
// type conversions. The function unit tests don't cover those things because
// they call directly into the functions.
//
// With that said then, this test function should contain at least one simple
// test case per function registered in the functions table (just to prove
// it really is registered correctly) and possibly a small set of additional
// functions showing real-world use-cases that rely on type conversion
// behaviors.
func TestFunctions(t *testing.T) {
	tests := []struct {
		src  string
		want cty.Value
	}{
		// Please maintain this list in alphabetical order by function, with
		// a blank line between the group of tests for each function.

		{
			`abs(-1)`,
			cty.NumberIntVal(1),
		},

		{
			`contains(["a", "b"], "a")`,
			cty.True,
		},
		{ // Should also work with sets, due to automatic conversion
			`contains(toset(["a", "b"]), "a")`,
			cty.True,
		},

		{
			`file("hello.txt")`,
			cty.StringVal("hello!"),
		},
	}

	for _, test := range tests {
		t.Run(test.src, func(t *testing.T) {
			expr, parseDiags := hclsyntax.ParseExpression([]byte(test.src), "test.hcl", hcl.Pos{Line: 1, Column: 1})
			if parseDiags.HasErrors() {
				for _, diag := range parseDiags {
					t.Error(diag.Error())
				}
				return
			}

			data := &dataForTests{} // no variables available; we only need literals here
			scope := &Scope{
				Data:    data,
				BaseDir: "./testdata/functions-test", // for the functions that read from the filesystem
			}

			got, diags := scope.EvalExpr(expr, cty.DynamicPseudoType)
			if diags.HasErrors() {
				for _, diag := range diags {
					t.Errorf("%s: %s", diag.Description().Summary, diag.Description().Detail)
				}
				return
			}

			if !test.want.RawEquals(got) {
				t.Errorf("wrong result\nexpr: %s\ngot:  %#v\nwant: %#v", test.src, got, test.want)
			}
		})
	}
}
