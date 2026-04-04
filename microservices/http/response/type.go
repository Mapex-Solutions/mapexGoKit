package response

// Response representa o formato padrão de resposta.
type Response struct {
	Status int         `json:"status"`
	Errors []string    `json:"errors"`
	Data   interface{} `json:"data"`
}
