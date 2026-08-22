# Spec: Default response description (OpenAPI)

## Summary

`internal/openapi/generate.go`'s `buildResponses` hoje só usa
`http.StatusText(status)` como description default pro caminho de ERRO
(status ≥ 400 sem schema explícito, via `defaultErrorResponse`). Todo outro
caso — status de sucesso (< 400) documentado via `r.Response(status, ...)`,
E o caminho sintetizado quando a rota nunca chama `Response()` (usa
`r.Code()`) — cai num `"description": ""` literal, vazio. Achado real
(screenshot do Swagger UI gerado, `POST /auth/register`): a resposta `201`
aparece sem NENHUMA descrição na UI, porque nunca há `.Description()`
chamado e o default pra esse caminho é string vazia.

Fix: TODO status, com ou sem schema, com ou sem erro, ganha
`http.StatusText(status)` como description default (`201` → `"Created"`,
`200` → `"OK"`, `404` → `"Not Found"`, etc — igual ao Swagger/OpenAPI de
outros frameworks, incluindo o comportamento NestJS `@Response()` que o
gonest já mira). `response.Description(...)` continua sobrescrevendo,
comportamento inalterado (já documentado no doc-comment de `Response.
Description`).

## Requirements

- REQ-001: rota SEM nenhum `r.Response(...)` chamado — a description
  sintetizada (hoje `""`, chave `r.Code()`) passa a ser
  `http.StatusText(r.Code())`.
- REQ-002: rota COM `r.Response(status, ...)` chamado mas SEM
  `.Description(...)` dentro do callback — a description passa a ser
  `http.StatusText(status)`, tanto pra status < 400 quanto ≥ 400 (o
  caminho de erro já fazia isso via `defaultErrorResponse`; sucesso não
  fazia).
- REQ-003: rota COM `.Description(...)` explícito continua usando
  exatamente esse texto — nenhuma mudança de comportamento aqui
  (`resp.DescriptionText()`'s `ok` bool já é a fonte da verdade, só
  precisa continuar sendo a última palavra depois do novo default).
- REQ-004: `http.StatusText` pra um status desconhecido/customizado (fora
  da tabela do `net/http`) retorna `""` — mesmo comportamento de hoje pra
  esses casos, sem regressão (não é um caso novo, só não piora).

## Affected Components (from graph)

- `buildResponses()` [`internal/openapi/generate.go:197`] — os 2 pontos
  que hoje hardcodeiam `"description": ""`: a síntese de rota sem
  `Response()` (linha ~202) e o branch de status-com-schema-ou-sucesso
  (linha ~214).
- `defaultErrorResponse()` [`internal/openapi/generate.go:240`] — já
  correto (usa `http.StatusText(status)`), fica como referência do
  comportamento a espelhar, sem mudança nele próprio.
- `route.Response.DescriptionText()` [`internal/route/response.go:57`] —
  consumido sem mudança, já é o mecanismo de override.

## Out of Scope

- Nenhuma mudança em `route.Response`/`route.Route` — é só o gerador
  (`internal/openapi`) que muda.
- Nenhuma tradução/i18n da description default — `http.StatusText` é
  sempre en-US (stdlib), igual o resto do gonest já assume pro OpenAPI
  gerado.

## Open Questions

Nenhuma — causa raiz confirmada lendo o código, fix é mecânico (mesma
função `http.StatusText` já usada no caminho de erro, só falta aplicar
nos outros 2 caminhos).
