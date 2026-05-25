// Package fga is a shared OpenFGA client wrapper and role/relation registry.
//
// Both user-svc and asset-svc import this package so the FGA authorization
// model, tuple subject conventions, and permission/role constants stay in
// one place. Per SCRUM-120 (user_id unification), all tuple writes go
// through the typed subject helpers here — never concatenate "user:" or
// "project:" prefixes directly in service code.
package fga

import "time"

// TupleRequest is the write-side payload for WriteTuple/WriteTuples.
type TupleRequest struct {
	User     string
	Relation string
	Object   string
}

// TupleRecord is a tuple returned from the Read API.
type TupleRecord struct {
	User      string
	Relation  string
	Object    string
	Timestamp time.Time
}

// ReadFilter narrows a Read call. Leave a field empty to match any value.
type ReadFilter struct {
	User     string
	Relation string
	Object   string
}

// CheckRequest is one entry in a BatchCheck call.
type CheckRequest struct {
	Relation string
	Object   string
}
