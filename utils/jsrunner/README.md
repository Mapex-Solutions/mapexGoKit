# jsrunner — Sandboxed JS transform runner (goja)

Runs a small JavaScript ES5 transform against an HTTP-response payload to produce a `[{label, value}]` array. Built on [`dop251/goja`](https://github.com/dop251/goja) so it does not need a Node.js process, and is bound by a context-based timeout to keep one bad script from stalling the request.

> Package name: `jsrunner` (directory: `jsrunner/`).

## Surface

```go
const DefaultTimeout = 10 * time.Second

type LabelValue struct {
    Label string      `json:"label"`
    Value interface{} `json:"value"`
}

func RunTransform(ctx context.Context, script string, data interface{}) ([]LabelValue, error)
```

### `RunTransform`

1. Builds a fresh `goja.Runtime` (no shared state across calls).
2. Sets the global JS variable `data` to the supplied Go value (goja maps maps→objects, slices→arrays, scalars→primitives).
3. Computes a timeout — `DefaultTimeout` unless the supplied `context.Context` already has a shorter deadline. A `time.AfterFunc` then calls `vm.Interrupt("execution timeout")` when the budget is exhausted.
4. Concatenates `<script>\n;transform(data);` and evaluates it. The script must define a top-level `function transform(data) { … }` returning an array of `{label, value}` objects.
5. Asserts the result is `[]interface{}` of objects with at least a `value` field. The `label` is converted via `fmt.Sprintf("%v", label)` (so missing labels become the literal string `"<nil>"`); a missing `value` aborts with an error.

Errors from steps 1–5 are wrapped with the `jsrunner: …` prefix.

## Usage

```go
script := `
  function transform(data) {
    return data.result.map(function(item) {
      return { label: item.title, value: item.id };
    });
  }
`

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

opts, err := jsrunner.RunTransform(ctx, script, response)
if err != nil { return err }
// opts is []jsrunner.LabelValue
```

## Notes

- The script runs without access to `require`, network, or filesystem — goja is pure JS evaluation.
- Each call creates a new VM. Hot paths benefit from caching the *script*, not the runtime — re-running a precompiled script in a fresh VM is the safe default.
- Timeout precision: the `time.AfterFunc` calls `vm.Interrupt`, which goja honours at the next bytecode step; tight loops without I/O can briefly exceed the deadline.
