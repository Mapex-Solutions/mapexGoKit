package requestValidation

import "reflect"

type Validation struct {
	bodyType   reflect.Type
	queryType  reflect.Type
	paramsType reflect.Type
}

/*
DTOTransformer is an opt-in contract for DTOs that want to run a post-validation
normalization step (e.g., enforce cross-field invariants, compute derived values,
or normalize payload shapes). Keeping it context-free makes it easy to unit test
and re-use outside HTTP.
*/
type DTOTransformer interface {
	// Transform runs AFTER defaults.Set and validator.Struct succeed.
	// It may mutate the receiver (normalize/derive) and should be idempotent.
	// Return an error to fail the request with a 400 response.
	Transform() error
}
