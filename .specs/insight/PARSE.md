`T` pode ser o struct puro (`Parse[Config]`) ou um ponteiro pra ele (`Parse[*Config]`) -- nesse segundo caso, `Parse` aloca o pointee via `reflect.New` e devolve o ponteiro já preenchido, sem precisar de um `config := Parse[Config](...); return &config` manual no call site.

```go
package ex

import "gonest.dev/gonest"

// precisa incluir um parse pro headers também
type Headers struct { XApiKey string `header:"x-api-key"` }
var HeadersSchema = gonest.NewSchema[Headers](func (t *Headers, s *gonest.Schema) {
  s.Property(&t.XApiKey).Description("X API Key")
})

// já é parseado por gonest.MustParse com a tag param
type Params struct { UserID string `param:"user_id"` }
var ParamsSchema = gonest.NewSchema[Params](func (t *Params, s *gonest.Schema) {
  s.Property(&t.UserID).Description("User ID")
})

// já é parseado por gonest.MustParse com a tag query
type Query struct { Limit  string `query:"limit"` }
var QuerySchema = gonest.NewSchema[Query](func (t *Query, s *gonest.Schema) {
  s.Property(&t.Limit).Description("Limit")
})

// já é parseado por gonest.MustParse com a tag json
type BodyJson struct { Name string `json:"name"` }
var BodyJsonSchema = gonest.NewSchema[BodyJson](func (t *BodyJson, s *gonest.Schema) {
  s.Property(&t.Name).Description("Name")
})

// já é parseado por gonest.MustParse com a tag form
type BodyForm struct { Name string `form:"name"` }
var BodyFormSchema = gonest.NewSchema[BodyForm](func (t *BodyForm, s *gonest.Schema) {
  s.Property(&t.Name).Description("Name")
})

var Controller = gonest.NewController(func(controller *gonest.Controller) {
  controller.Path("/ex")
  controller.Route(gonest.HttpPatch, "/", func (r *gonest.Route) {
    r.Handler(func (req *gonest.Request, res *gonest.Response) {
      // headers, err := gonest.Parse[Headers](req.Headers(), HeadersSchema)
      headers := gonest.MustParse[Headers](req.Headers(), HeadersSchema)
      // params, err := gonest.Parse[Params](req.Params(), ParamsSchema)
      params := gonest.MustParse[Params](req.Params(), ParamsSchema)
      // query, err := gonest.Parse[Query](req.Query(), QuerySchema)
      query := gonest.MustParse[Query](req.Query(), QuerySchema)
      // bodyJson, err := gonest.Parse[BodyJson](req.Body().Json(), BodyJsonSchema)
      bodyJson := gonest.MustParse[BodyJson](req.Body().Json(), BodyJsonSchema)
      // bodyForm, err := gonest.Parse[BodyForm](req.Body().Form(nil), BodyFormSchema)
      bodyForm := gonest.MustParse[BodyForm](req.Body().Form(nil), BodyFormSchema)
      bodyRaw := req.Body().Raw()
      bodyText := req.Body().Text()
      _, _, _, _, _, _, _ = headers, params, query, bodyJson, bodyForm, bodyRaw, bodyText
    })
  })
})

```
