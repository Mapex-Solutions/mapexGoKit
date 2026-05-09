# jsrunner — Runner JS sandboxed para transforms (goja)

Roda um pequeno transform JavaScript ES5 contra um payload de resposta HTTP para produzir um array `[{label, value}]`. Construído em cima de [`dop251/goja`](https://github.com/dop251/goja) para não precisar de processo Node.js, e limitado por timeout via context para que um script ruim não trave o request.

> Nome do pacote: `jsrunner` (diretório: `jsrunner/`).

## Superfície

```go
const DefaultTimeout = 10 * time.Second

type LabelValue struct {
    Label string      `json:"label"`
    Value interface{} `json:"value"`
}

func RunTransform(ctx context.Context, script string, data interface{}) ([]LabelValue, error)
```

### `RunTransform`

1. Constrói um `goja.Runtime` novo (sem estado compartilhado entre chamadas).
2. Seta a variável JS global `data` com o valor Go informado (goja mapeia maps→objects, slices→arrays, escalares→primitivos).
3. Calcula o timeout — `DefaultTimeout` exceto se o `context.Context` informado já tem deadline menor. Um `time.AfterFunc` então chama `vm.Interrupt("execution timeout")` quando o orçamento esgota.
4. Concatena `<script>\n;transform(data);` e avalia. O script deve definir uma `function transform(data) { … }` no topo que retorna array de objetos `{label, value}`.
5. Verifica que o resultado é `[]interface{}` de objetos com pelo menos um field `value`. O `label` é convertido via `fmt.Sprintf("%v", label)` (então labels ausentes viram a string literal `"<nil>"`); um `value` ausente aborta com erro.

Erros dos passos 1–5 são embrulhados com o prefixo `jsrunner: …`.

## Uso

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
// opts é []jsrunner.LabelValue
```

## Notas

- O script roda sem acesso a `require`, rede ou filesystem — goja é avaliação JS pura.
- Cada chamada cria uma VM nova. Hot paths se beneficiam de cachear o *script*, não o runtime — re-rodar um script pré-compilado em VM nova é o default seguro.
- Precisão do timeout: o `time.AfterFunc` chama `vm.Interrupt`, que goja honra no próximo step de bytecode; loops apertados sem I/O podem estourar o deadline brevemente.
