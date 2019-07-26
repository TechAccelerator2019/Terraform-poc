package cliconfigupgrade

import (
	hcl1ast "github.com/hashicorp/hcl/hcl/ast"

	hcl2syntax "github.com/hashicorp/hcl2/hcl/hclsyntax"
	hcl2write "github.com/hashicorp/hcl2/hclwrite"
)

func writeComments(to *hcl2write.Body, comments *hcl1ast.CommentGroup) {
	if comments == nil {
		return
	}
	tokens := make(hcl2write.Tokens, 0, len(comments.List)*2)

	for _, comment := range comments.List {
		tokens = append(tokens, &hcl2write.Token{
			Type:  hcl2syntax.TokenComment,
			Bytes: []byte(comment.Text),
		})
		tokens = append(tokens, &hcl2write.Token{
			Type:  hcl2syntax.TokenNewline,
			Bytes: []byte{'\n'},
		})
	}

	to.AppendUnstructuredTokens(tokens)
}

func writeNewline(to *hcl2write.Body) {
	to.AppendNewline()
}
