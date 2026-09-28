// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package interleaved

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
)

// LookupVar returns the package-level variable symName from package
// pkg. It returns an error if the variable is not in the export data.
var LookupVar = func(pkg *types.Pkg, symName string) (*ir.Name, error) {
	base.Fatalf("interleaved.LookupVar not overridden")
	panic("unreachable")
}

// rewriteAddrFrom16As16 replaces netip.AddrFrom16(x.As16()) with
// netip.Addr{addr: x.addr, z: netip.z6noz}. It returns nil if n does
// not match.
func rewriteAddrFrom16As16(n ir.Node) ir.Node {
	if base.Flag.LowerL == 0 {
		return nil
	}
	call, ok := n.(*ir.CallExpr)
	if !ok || call.Op() != ir.OCALLFUNC || len(call.Args) != 1 || len(call.Init()) != 0 ||
		!isNetipFunc(call.Fun, "AddrFrom16") {
		return nil
	}
	arg := call.Args[0]
	if p, ok := arg.(*ir.ParenExpr); ok {
		arg = p.X
	}
	inner, ok := arg.(*ir.CallExpr)
	if !ok || inner.Op() != ir.OCALLFUNC || len(inner.Args) != 1 || len(inner.Init()) != 0 ||
		!isNetipFunc(inner.Fun, "Addr.As16") {
		return nil
	}

	// The result of an inlined call must not have side effects.
	x := inner.Args[0]
	if !isVarOrField(x) {
		return nil
	}

	typ := call.Type()
	z6noz, err := LookupVar(ir.StaticCalleeName(call.Fun).Sym().Pkg, "z6noz")
	if err != nil {
		return nil
	}

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

	// An inlined call is valid both as an expression and as a
	// statement.
	res := ir.NewInlinedCallExpr(pos, nil, []ir.Node{lit})
	res.SetType(typ)
	res.SetTypecheck(1)

	if base.Flag.LowerM != 0 {
		base.WarnfAt(pos, "rewriting netip.AddrFrom16(x.As16())")
	}
	return res
}

func isNetipFunc(fun ir.Node, name string) bool {
	fn := ir.StaticCalleeName(fun)
	return fn != nil && fn.Sym().Pkg.Path == "net/netip" && fn.Sym().Name == name
}

func isVarOrField(n ir.Node) bool {
	for {
		switch n.Op() {
		case ir.ONAME:
			return true
		case ir.ODOT:
			n = n.(*ir.SelectorExpr).X
		default:
			return false
		}
	}
}
