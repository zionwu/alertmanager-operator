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

	versionsHandler := api.VersionsHandler(schemas, "v1")
	versionHandler := api.VersionHandler(schemas, "v1")
	r.Methods(http.MethodGet).Path("/").Handler(versionsHandler)
	r.Methods(http.MethodGet).Path("/v1").Handler(versionHandler)
	r.Methods(http.MethodGet).Path("/v1/apiversions").Handler(versionsHandler)
	r.Methods(http.MethodGet).Path("/v1/apiversions/v1").Handler(versionHandler)
	r.Methods(http.MethodGet).Path("/v1/schemas").Handler(api.SchemasHandler(schemas))
	r.Methods(http.MethodGet).Path("/v1/schemas/{id}").Handler(api.SchemaHandler(schemas))

	r.Methods(http.MethodGet).Path("/v1/notifiers").Handler(f(schemas, s.NotifiersList))
	r.Methods(http.MethodGet).Path("/v1/notifier").Handler(f(schemas, s.NotifiersList))
	r.Methods(http.MethodPost).Path("/v1/notifiers").Handler(f(schemas, s.CreateNotifier))
	r.Methods(http.MethodPost).Path("/v1/notifier").Handler(f(schemas, s.CreateNotifier))
	r.Methods(http.MethodGet).Path("/v1/notifiers/{id}").Handler(f(schemas, s.GetNotifier))
	r.Methods(http.MethodDelete).Path("/v1/notifiers/{id}").Handler(f(schemas, s.DeleteNotifier))
	r.Methods(http.MethodPut).Path("/v1/notifiers/{id}").Handler(f(schemas, s.UpdateNotifier))

	r.Methods(http.MethodGet).Path("/v1/recipient").Handler(f(schemas, s.RecipientsList))
	r.Methods(http.MethodGet).Path("/v1/recipients").Handler(f(schemas, s.RecipientsList))
	r.Methods(http.MethodPost).Path("/v1/recipients").Handler(f(schemas, s.CreateRecipient))
	r.Methods(http.MethodPost).Path("/v1/recipients").Handler(f(schemas, s.CreateRecipient))
	r.Methods(http.MethodGet).Path("/v1/recipients/{id}").Handler(f(schemas, s.GetRecipient))
	r.Methods(http.MethodDelete).Path("/v1/recipients/{id}").Handler(f(schemas, s.DeleteRecipient))
	r.Methods(http.MethodPut).Path("/v1/recipients/{id}").Handler(f(schemas, s.UpdateRecipient))

	r.Methods(http.MethodGet).Path("/v1/alert").Handler(f(schemas, s.AlertsList))
	r.Methods(http.MethodGet).Path("/v1/alerts").Handler(f(schemas, s.AlertsList))
	r.Methods(http.MethodPost).Path("/v1/alert").Handler(f(schemas, s.CreateAlert))
	r.Methods(http.MethodPost).Path("/v1/alerts").Handler(f(schemas, s.CreateAlert))
	r.Methods(http.MethodGet).Path("/v1/alerts/{id}").Handler(f(schemas, s.GetAlert))
	r.Methods(http.MethodDelete).Path("/v1/alerts/{id}").Handler(f(schemas, s.DeleteAlert))
	r.Methods(http.MethodPut).Path("/v1/alerts/{id}").Handler(f(schemas, s.UpdateAlert))

	return r
}
