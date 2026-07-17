```go
package ex

import "gonest.dev/gonest"

// precisa incluir um parse pro headers também
type Headers struct { XApiKey string `header:"x-api-key"` }
var HeadersSchema = gonest.Schema[Headers](func (t *Headers, s *gonest.Schema) {
  s.Property(&t.XApiKey).Description("X API Key")
})

// já é parseado pelo MustParseRestParams com a tag param
type Params struct { UserID string `param:"user_id"` }
var ParamsSchema = gonest.Schema[Params](func (t *Params, s *gonest.Schema) {
  s.Property(&t.UserID).Description("User ID")
})

// já é parseado pelo MustParseRestQuery com a tag query
type Query struct { Limit  string `query:"limit"` }
var QuerySchema = gonest.Schema[Query](func (t *Query, s *gonest.Schema) {
  s.Property(&t.Limit).Description("Limit")
})

// já é parseado pelo MustParseRestJsonBody com a tag json
type BodyJson struct { Name string `json:"name"` }
var BodyJsonSchema = gonest.Schema[BodyJson](func (t *BodyJson, s *gonest.Schema) {
  s.Property(&t.Name).Description("Name")
})

// já é parseado pelo MustParseRestForm com a tag form
type BodyForm struct { Name string `form:"name"` }
var BodyFormSchema = gonest.Schema[BodyForm](func (t *BodyForm, s *gonest.Schema) {
  s.Property(&t.Name).Description("Name")
})

var Controller = gonest.NewController(func(controller *gonest.Controller) {
  controller.Path("/ex")
  controller.Route(gonest.HttpPatch, "/", func (r *gonest.Route) {
    r.Handler(func (ctx *gonest.RestContext) {
      // headers, err := gonest.Parse[Headers](ctx.Headers(), HeadersSchema)
      headers := gonest.MustParse[Headers](ctx.Headers(), HeadersSchema)
      // params, err := gonest.Parse[Params](ctx.Params(), ParamsSchema)
      params := gonest.MustParse[Params](ctx.Params(), ParamsSchema)
      // query, err := gonest.Parse[Query](ctx.Query(), QuerySchema)
      query := gonest.MustParse[Query](ctx.Query(), QuerySchema)
      // bodyJson, err := gonest.Parse[BodyJson](ctx.Body().Json(), BodyJsonSchema)
      bodyJson := gonest.MustParse[BodyJson](ctx.Body().Json(), BodyJsonSchema)
      // bodyForm, err := gonest.Parse[BodyForm](ctx.Body().Form(nil), BodyFormSchema)
      bodyForm := gonest.MustParse[BodyForm](ctx.Body().Form(nil), BodyFormSchema)
      _, _, _, _, _ = headers, params, query, bodyJson, bodyForm
    })
  })
})

```