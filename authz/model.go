package authz

import _ "embed"

// The model travels with the package so a service and the store it talks to
// cannot disagree about what `can_edit` means. model.fga is the readable
// source; model.json is what the API accepts.
//
//go:embed model.fga
var modelDSL string

//go:embed model.json
var modelJSON []byte

// ModelDSL returns the authorization model in OpenFGA's DSL, for humans and
// for `fga model write --file`.
func ModelDSL() string { return modelDSL }

// ModelJSON returns the authorization model as the JSON the
// POST /stores/{id}/authorization-models endpoint expects. Callers that
// bootstrap a store should upload this rather than reading a file from disk,
// which is how a deployed binary ends up writing a model it was not built
// against.
func ModelJSON() []byte {
	out := make([]byte, len(modelJSON))
	copy(out, modelJSON)
	return out
}
