// Package ast defines the arena-backed Abstract Syntax Tree used by the Rahu
// Python frontend.
//
// The AST is implemented as a flat arena of nodes stored in a contiguous slice.
// Each node is identified by a NodeID, which corresponds to the index of the
// node in AST.Nodes. NodeID 0 is reserved as the sentinel value NoNode.
//
// Instead of allocating distinct Go structs for every syntactic construct, the
// tree uses a compact tagged-node representation:
//
//   - Node.Kind identifies the type of syntax node.
//   - Node.Data stores payload information whose meaning depends on Kind
//     (e.g. operator enums or indices into literal tables).
//   - Node.Start and Node.End store byte offsets into the original source.
//   - Node.FirstChild and Node.NextSibling encode the tree structure.
//
// Children are represented as a singly linked list. FirstChild points to the
// first child node and each child links to the next via NextSibling. This keeps
// nodes fixed-size and avoids per-node allocations.
//
// Literal and identifier text are stored outside the node arena in side tables
// (AST.Names, AST.Strings, AST.Numbers). Nodes reference these tables through
// the Data field. This design avoids repeated string allocations and allows
// the AST to remain compact and cache-friendly.
//
// Nodes are created through AST.NewNode and connected using AST.AddChild.
// The arena grows automatically through slice append and node IDs remain
// stable for the lifetime of the AST.
//
// This layout trades structural Go types for a uniform node representation,
// improving memory locality and enabling efficient traversal and analysis
// passes in later compiler stages.
package ast
