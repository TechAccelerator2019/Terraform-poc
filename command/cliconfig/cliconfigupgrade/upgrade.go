package cliconfigupgrade

import (
	"fmt"

	hcl1ast "github.com/hashicorp/hcl/hcl/ast"
	hcl1parser "github.com/hashicorp/hcl/hcl/parser"

	hcl2write "github.com/hashicorp/hcl2/hclwrite"

	"github.com/zclconf/go-cty/cty"
)

// UpgradeOldHCLConfig produces a buffer that is functionally equivalent to the
// input when read by the HCL 1.0 CLI Config implementation used in prior
// Terraform versions, but can also be parsed successfully by the HCL 2.0-based
// CLI Config implementation we're now using.
//
// This function will prefer to return the given buffer verbatim if it can prove
// that it has equivalent meaning to both parsers, or if it has syntax errors
// that would cause parsing to fail in both old and new loader implementations.
func UpgradeOldHCLConfig(old []byte) []byte {

	// Re-iterating the statement from the public doc comment above: it's
	// important that this function always produce something that would
	// have equivalent behavior for older Terraform versions that are using
	// the old parser, because CLI config is shared between all versions of
	// Terraform run by a particular user.
	//
	// The CLI config loaders are designed to ignore constructs they don't
	// understand, so as long as the overall syntax is valid enough for
	// both implementations to read successfully our main focus is on making
	// sure the constructs that older versions _do_ support remain compatible:
	// * The "providers" and "provisioners" maps.
	// * The "plugin_cache_dir" string argument.
	// * "host" blocks and their "services" nested map argument.
	// * "credentials" blocks and their "token" nested map argument.
	// * "credentials_helper" blocks and their "args" nested list argument.
	//
	// Note that only the first to of these include environment variable
	// expansion. The old configuration loader treated every other string
	// value totally literally, and so we must preserve that meaning when
	// translating.
	//
	// One unfortunate compatibility exception is that there is no way for us
	// to preserve a literal `${ ... }` sequence in an input string in a way
	// that won't be interpreted as `$${ ... }` by the old parser. We're
	// accepting that caveat on the assumption that none of the features above
	// have a stark need for literal `${ ... }` sequences to be included and
	// thus it's highly unlikely that this will turn up in any non-contrived
	// situation.

	oldF, err := hcl1parser.Parse(old)
	if err != nil {
		// If it can't even be parsed by the HCL 1.0 parser (generally more
		// liberal than the HCL 2.0 one) then we'll just leave it unchanged
		// and let the caller return a HCL 2.0-style syntax error.
		return old
	}

	if !configNeedsUpgrade(oldF) {
		// We'll prefer to leave files completely unchanged if possible. This
		// should be the common case, unless the given file was using the
		// old-style naked environment variable syntax $FOO, or if the
		// input is using configuration forms that HCL 1.0 would allow but
		// HCL 2.0 will not.
		return old
	}

	adhocComments := collectAdhocComments(oldF)

	oldRootList := oldF.Node.(*hcl1ast.ObjectList)

	newF := hcl2write.NewEmptyFile()
	newBody := newF.Body()

	upgradeBody(oldRootList, newBody, adhocComments, true)

	return newF.Bytes()
}

func configNeedsUpgrade(old *hcl1ast.File) bool {
	// TODO: Implement this heuristic
	return true
}

func upgradeBody(from *hcl1ast.ObjectList, to *hcl2write.Body, adhocComments *commentQueue, root bool) {
	items := from.Items

	for i, item := range items {
		comments := adhocComments.TakeBefore(item)
		for _, group := range comments {
			writeComments(to, group)
			writeNewline(to) // Extra separator after each group
		}

		writeComments(to, item.LeadComment)

		name := item.Keys[0].Token.Value().(string)

		// Did the old HCL 1.0-based parser use the "ExpandEnv" behavior
		// for this argument?
		oldDidExpandEnv := false
		if root {
			switch name {
			case "providers", "provisioners", "plugin_cache_dir":
				oldDidExpandEnv = true
			}
		}

		// Does the given item use argument syntax or block syntax? We'll
		// keep it the same here.
		blockSyntax := item.Assign.Line == 0 || len(item.Keys) > 1
		if _, ok := item.Val.(*hcl1ast.ObjectType); !ok {
			// Should never happen if blockSyntax is already true since
			// omitting the equals or having labels is only valid when
			// followed by a brace to introduce an object, but we'll be robust
			// about it and force this anyway.
			blockSyntax = true
		}

		switch {
		case oldDidExpandEnv:
			// All of the "ExpandEnv"-like cases in the new loader use
			// HCL 2.0 argument syntax, so we'll force that here.
			upgradeArgument(item, to, true)
		case blockSyntax:
			upgradeNestedBlock(item, to, adhocComments)
		default:
			upgradeArgument(item, to, false)
		}

		// If we have another item and it's more than one line away
		// from the current one then we'll print an extra blank line
		// to retain that separation.
		if (i + 1) < len(items) {
			next := items[i+1]
			thisPos := hcl1NodeEndPos(item)
			nextPos := next.Pos()
			if nextPos.Line-thisPos.Line > 1 {
				writeNewline(to)
			}
		}

	}
}

func upgradeArgument(from *hcl1ast.ObjectItem, to *hcl2write.Body, expandEnv bool) {
	name := from.Keys[0].Token.Value().(string)

	if expandEnv {
		// TODO: This case is harder, because we need to produce a compound
		// expression and hclwrite doesn't currently support that.
		panic("ExpandEnv expression upgrade not yet supported")
	}

	val := upgradeExpressionConstant(from.Val)

	// Note: if the input has multiple definitions of the same attribute name
	// then this will keep only the last one, because hclwrite automatically
	// preserves the HCL 2.0 requirement that each argument be defined only
	// once. This doesn't actually change the outcome for old parsers, because
	// HCL 1.0 would take the final definition as the actual result in that
	// case anyway.
	to.SetAttributeValue(name, val)

	// Note: this doesn't preserve the formatting exactly, because hclwrite
	// doesn't currently have a mechanism for generating end-of-line comments
	// where they weren't already present, but we will at least preserve the
	// comment content.
	writeComments(to, from.LineComment)
}

func upgradeNestedBlock(from *hcl1ast.ObjectItem, to *hcl2write.Body, adhocComments *commentQueue) {
	name := from.Keys[0].Token.Value().(string)
	labels := make([]string, len(from.Keys)-1)
	for i := range labels {
		labels[i] = from.Keys[i+1].Token.Value().(string)
	}
	newBlock := to.AppendNewBlock(name, labels)
	newBody := newBlock.Body()

	oldObjectList := from.Val.(*hcl1ast.ObjectType).List

	upgradeBody(oldObjectList, newBody, adhocComments, false)
}

func upgradeExpressionConstant(from hcl1ast.Node) cty.Value {
	switch n := from.(type) {

	case *hcl1ast.LiteralType:
		raw := n.Token.Value()
		switch v := raw.(type) {
		case string:
			// Note: we're losing whether or not this is a "heredoc" string.
			// The pre-HCL 2 config format contained no multi-line string
			// values, so it would be surprising to see a "heredoc" there, and
			// if we do then converting to a normal string shouldn't hurt.
			return cty.StringVal(v)
		case int:
			return cty.NumberIntVal(int64(v))
		case float64:
			return cty.NumberFloatVal(v)
		case bool:
			return cty.BoolVal(v)
		default:
			// Should never happen, since the above covers all the types that
			// Token.Value ought to return.
			panic(fmt.Sprintf("cannot derive expression from %T", raw))
		}

	case *hcl1ast.ListType:
		vals := make([]cty.Value, len(n.List))
		for i, node := range n.List {
			// For simplicity, we're not going hunting for comments embedded
			// inside the compound list expression. That means that any
			// comments inside will get pushed out to the end of the value.
			// That's unfortunate, but since we will only be performing an
			// upgrade when it's totally unavoidable it's an acceptable
			// compromise.
			vals[i] = upgradeExpressionConstant(node)
		}
		return cty.TupleVal(vals)

	case *hcl1ast.ObjectType:
		vals := make(map[string]cty.Value)
		for _, item := range n.List.Items {
			// For simplicity, we're not going hunting for comments embedded
			// inside the compound object expression. That means that any
			// comments inside will get pushed out to the end of the value.
			// That's unfortunate, but since we will only be performing an
			// upgrade when it's totally unavoidable it's an acceptable
			// compromise.
			//
			// We'll also lose any duplicate keys in this process, but that's
			// okay because HCL 1.0 would only have considered the last one
			// of each name anyway.
			name := item.Keys[0].Token.Value().(string)
			vals[name] = upgradeExpressionConstant(item.Val)
		}
		return cty.ObjectVal(vals)

	case *hcl1ast.File, *hcl1ast.Comment, *hcl1ast.CommentGroup, *hcl1ast.ObjectKey, *hcl1ast.ObjectItem:
		// These can never represent an expression. No HCL 1.0 input can
		// possibly put these types in an expression context, so it must be
		// a bug in the upgrade logic.
		panic(fmt.Sprintf("cannot derive expression from %T", from))

	default:
		// Should never happen because the above cases exhaustively cover all
		// of the hcl1ast Node implementations.
		panic(fmt.Sprintf("unsupported HCL 1.0 node type %T", from))
	}
}
