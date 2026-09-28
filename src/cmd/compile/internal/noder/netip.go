// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
	"cmd/internal/src"
)

// rewriteAddrFrom16As16 replaces netip.AddrFrom16(x.As16()) with
// netip.Addr{addr: x.addr, z: netip.z6noz}. It returns nil if n does
// not match. The result is not a call. When n is used as a statement,
// use discardAddrFrom16As16 on the result.
func rewriteAddrFrom16As16(n ir.Node) ir.Node {
	if base.Flag.LowerL == 0 {
		return nil
	}
	call, ok := n.(*ir.CallExpr)
	if !ok || call.Op() != ir.OCALLFUNC || len(call.Args) != 1 || len(call.Init()) != 0 ||
		!isNetipFunc(call.Fun, "AddrFrom16") {
		return nil
	}
	inner, ok := call.Args[0].(*ir.CallExpr)
	if !ok || inner.Op() != ir.OCALLFUNC || len(inner.Args) != 1 || len(inner.Init()) != 0 ||
		!isNetipFunc(inner.Fun, "Addr.As16") {
		return nil
	}

	x := inner.Args[0]
	z6noz, err := lookupVar(ir.StaticCalleeName(call.Fun).Sym().Pkg, "z6noz")
	if err != nil {
		return nil
	}

	typ := call.Type()
	pos := call.Pos()
	var list []ir.Node
	for i, f := range typ.Fields() {
		var value ir.Node
		switch f.Sym.Name {
		case "addr":
			value = typecheck.DotField(pos, x, i)
		case "z":
			value = z6noz
		default:
			return nil
		}
		list = append(list, ir.NewStructKeyExpr(pos, f, value))
	}
	lit := ir.NewCompLitExpr(pos, ir.OSTRUCTLIT, typ, list)
	lit.SetTypecheck(1)

	if base.Flag.LowerM != 0 {
		base.WarnfAt(pos, "rewriting netip.AddrFrom16(x.As16())")
	}
	return lit
}

// discardAddrFrom16As16 turns the result of rewriteAddrFrom16As16 into
// an assignment to the blank identifier, to use it as a statement. A
// struct literal alone is not a valid statement, so a statement that is
// a struct literal can only come from rewriteAddrFrom16As16.
func discardAddrFrom16As16(pos src.XPos, lit ir.Node) ir.Node {
	return typecheck.Stmt(ir.NewAssignStmt(pos, ir.BlankNode, lit))
}

func isNetipFunc(fun ir.Node, name string) bool {
	fn := ir.StaticCalleeName(fun)
	return fn != nil && fn.Sym().Pkg.Path == "net/netip" && fn.Sym().Name == name
}
