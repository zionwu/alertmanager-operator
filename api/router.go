package api

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
)

type HandleFuncWithError func(http.ResponseWriter, *http.Request) error

func HandleError(s *client.Schemas, t HandleFuncWithError) http.Handler {
	return api.ApiHandler(s, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if err := t(rw, req); err != nil {
			logrus.Warnf("HTTP handling error %v", err)
			apiContext := api.GetApiContext(req)
			apiContext.Write(err)
		}
	}))
}

func NewRouter(s *Server) *mux.Router {
	schemas := NewSchema()
	r := mux.NewRouter().StrictSlash(true)
	f := HandleError

	versionsHandler := api.VersionsHandler(schemas, "v5")
	versionHandler := api.VersionHandler(schemas, "v5")
	r.Methods(http.MethodGet).Path("/v5/").Handler(versionsHandler)
	r.Methods(http.MethodGet).Path("/v5/monitoring").Handler(versionHandler)
	r.Methods(http.MethodGet).Path("/v5/apiversions").Handler(versionsHandler)
	r.Methods(http.MethodGet).Path("/v5/apiversions/v5").Handler(versionHandler)
	r.Methods(http.MethodGet).Path("/v5/schemas").Handler(api.SchemasHandler(schemas))
	r.Methods(http.MethodGet).Path("/v5/schemas/{id}").Handler(api.SchemaHandler(schemas))

	r.Methods(http.MethodGet).Path("/v5/notifiers").Handler(f(schemas, s.NotifiersList))
	r.Methods(http.MethodGet).Path("/v5/notifier").Handler(f(schemas, s.NotifiersList))
	r.Methods(http.MethodPost).Path("/v5/notifiers").Handler(f(schemas, s.CreateNotifier))
	r.Methods(http.MethodPost).Path("/v5/notifier").Handler(f(schemas, s.CreateNotifier))
	r.Methods(http.MethodGet).Path("/v5/notifiers/{id}").Handler(f(schemas, s.GetNotifier))
	r.Methods(http.MethodDelete).Path("/v5/notifiers/{id}").Handler(f(schemas, s.DeleteNotifier))
	r.Methods(http.MethodPut).Path("/v5/notifiers/{id}").Handler(f(schemas, s.UpdateNotifier))

	r.Methods(http.MethodGet).Path("/v5/recipient").Handler(f(schemas, s.RecipientsList))
	r.Methods(http.MethodGet).Path("/v5/recipients").Handler(f(schemas, s.RecipientsList))
	r.Methods(http.MethodPost).Path("/v5/recipients").Handler(f(schemas, s.CreateRecipient))
	r.Methods(http.MethodPost).Path("/v5/recipients").Handler(f(schemas, s.CreateRecipient))
	r.Methods(http.MethodGet).Path("/v5/recipients/{id}").Handler(f(schemas, s.GetRecipient))
	r.Methods(http.MethodDelete).Path("/v5/recipients/{id}").Handler(f(schemas, s.DeleteRecipient))
	r.Methods(http.MethodPut).Path("/v5/recipients/{id}").Handler(f(schemas, s.UpdateRecipient))

	r.Methods(http.MethodGet).Path("/v5/alert").Handler(f(schemas, s.AlertsList))
	r.Methods(http.MethodGet).Path("/v5/alerts").Handler(f(schemas, s.AlertsList))
	r.Methods(http.MethodPost).Path("/v5/alert").Handler(f(schemas, s.CreateAlert))
	r.Methods(http.MethodPost).Path("/v5/alerts").Handler(f(schemas, s.CreateAlert))
	r.Methods(http.MethodGet).Path("/v5/alerts/{id}").Handler(f(schemas, s.GetAlert))
	r.Methods(http.MethodDelete).Path("/v5/alerts/{id}").Handler(f(schemas, s.DeleteAlert))
	r.Methods(http.MethodPut).Path("/v5/alerts/{id}").Handler(f(schemas, s.UpdateAlert))

	return r
}
