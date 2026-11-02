package openapi

import (
	"net/http"

	"entgo.io/contrib/entgql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/vektah/gqlparser/v2/ast"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
)

type GraphqlHandler struct {
	Graphql    http.Handler
	Playground http.Handler
}

type Dependencies struct {
	fx.In

	Ent                          *ent.Client
	APIKeyService                *biz.APIKeyService
	APIKeyProfileTemplateService *biz.APIKeyProfileTemplateService
}

func NewGraphqlHandlers(deps Dependencies) *GraphqlHandler {
	gqlSrv := handler.New(NewSchema(deps.APIKeyService, deps.APIKeyProfileTemplateService))

	gqlSrv.AddTransport(transport.Options{})
	// Intentionally NOT registering transport.GET: the openapi schema accepts
	// plaintext API keys, and gqlgen's GET transport reads operation variables
	// from the URL query string. Allowing GET would let secret API keys land in
	// URLs (reverse-proxy/access logs, browser history, traces). Programmatic
	// clients (and the genqlient SDK) use POST, so GET has no legitimate
	// consumer here.
	gqlSrv.AddTransport(transport.POST{})
	gqlSrv.AddTransport(transport.MultipartForm{})

	gqlSrv.SetQueryCache(lru.New[*ast.QueryDocument](1024))

	gqlSrv.Use(extension.Introspection{})
	gqlSrv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](1024),
	})
	// gqlSrv.Use(&loggingTracer{})
	gqlSrv.Use(entgql.Transactioner{
		TxOpener: deps.Ent,
	})

	return &GraphqlHandler{
		Graphql:    gqlSrv,
		Playground: playground.Handler("AxonHub", "/openapi/v1/graphql"),
	}
}
