package zerovalue

// Ptr returns the address of v. It exists so callers can build payloads with
// optional pointer fields inline, without introducing a throwaway variable
// at every call site:
//
//	dto := AssetCreate{
//	    ThresholdMinutes: zerovalue.Ptr(10),
//	    Enabled:          zerovalue.Ptr(true),
//	}
//
// The helper is in the zerovalue package because both `Ptr(v)` and
// `GetZeroValue(...)` answer the same family of question — "give me the
// canonical value for a field whose type the caller cannot inline".
func Ptr[T any](v T) *T { return &v }
