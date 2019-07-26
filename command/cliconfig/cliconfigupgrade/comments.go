package cliconfigupgrade

import (
	"sort"

	hcl1ast "github.com/hashicorp/hcl/hcl/ast"
	hcl1token "github.com/hashicorp/hcl/hcl/token"
)

func collectAdhocComments(f *hcl1ast.File) *commentQueue {
	comments := make(map[hcl1token.Pos]*hcl1ast.CommentGroup)
	for _, c := range f.Comments {
		comments[c.Pos()] = c
	}

	// We'll remove from our map any comments that are attached to specific
	// nodes as lead or line comments, since we'll find those during our
	// walk anyway.
	hcl1ast.Walk(f, func(nn hcl1ast.Node) (hcl1ast.Node, bool) {
		switch t := nn.(type) {
		case *hcl1ast.LiteralType:
			if t.LeadComment != nil {
				for _, comment := range t.LeadComment.List {
					delete(comments, comment.Pos())
				}
			}

			if t.LineComment != nil {
				for _, comment := range t.LineComment.List {
					delete(comments, comment.Pos())
				}
			}
		case *hcl1ast.ObjectItem:
			if t.LeadComment != nil {
				for _, comment := range t.LeadComment.List {
					delete(comments, comment.Pos())
				}
			}

			if t.LineComment != nil {
				for _, comment := range t.LineComment.List {
					delete(comments, comment.Pos())
				}
			}
		}

		return nn, true
	})

	if len(comments) == 0 {
		var ret commentQueue
		return &ret
	}

	ret := make([]*hcl1ast.CommentGroup, 0, len(comments))
	for _, c := range comments {
		ret = append(ret, c)
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Pos().Before(ret[j].Pos())
	})
	queue := commentQueue(ret)
	return &queue
}

type commentQueue []*hcl1ast.CommentGroup

func (q *commentQueue) TakeBeforeToken(token hcl1token.Token) []*hcl1ast.CommentGroup {
	return q.TakeBeforePos(token.Pos)
}

func (q *commentQueue) TakeBefore(node hcl1ast.Node) []*hcl1ast.CommentGroup {
	return q.TakeBeforePos(node.Pos())
}

func (q *commentQueue) TakeBeforePos(pos hcl1token.Pos) []*hcl1ast.CommentGroup {
	toPos := pos
	var i int
	for i = 0; i < len(*q); i++ {
		if (*q)[i].Pos().After(toPos) {
			break
		}
	}
	if i == 0 {
		return nil
	}

	ret := (*q)[:i]
	*q = (*q)[i:]

	return ret
}

// hcl1NodeEndPos tries to find the latest possible position in the given
// node. This is primarily to try to find the last line number of a multi-line
// construct and is a best-effort sort of thing because HCL1 only tracks
// start positions for tokens and has no generalized way to find the full
// range for a single node.
func hcl1NodeEndPos(node hcl1ast.Node) hcl1token.Pos {
	switch tn := node.(type) {
	case *hcl1ast.ObjectItem:
		if tn.LineComment != nil && len(tn.LineComment.List) > 0 {
			return tn.LineComment.List[len(tn.LineComment.List)-1].Start
		}
		return hcl1NodeEndPos(tn.Val)
	case *hcl1ast.ListType:
		return tn.Rbrack
	case *hcl1ast.ObjectType:
		return tn.Rbrace
	default:
		// If all else fails, we'll just return the position of what we were given.
		return tn.Pos()
	}
}
