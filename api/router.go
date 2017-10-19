package api

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
)

type HandleFuncWithError func(http.ResponseWriter, *http.Request) error

//HandleError handle error from operation
func handleError(s *client.Schemas, t func(http.ResponseWriter, *http.Request) error) http.Handler {
	return api.ApiHandler(s, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if err := t(rw, req); err != nil {
			logrus.Errorf("Got Error: %v", err)
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(500)

			e := Error{
				Resource: client.Resource{
					Type: "error",
				},
				Status:   500,
				Msg:      err.Error(),
				BaseType: "error",
			}
			api.GetApiContext(req).Write(&e)
		}
	}))
}

func NewRouter(s *Server) *mux.Router {
	schemas := newSchema()
	r := mux.NewRouter().StrictSlash(true)
	f := handleError

	versionsHandler := api.VersionsHandler(schemas, "v5")
	versionHandler := api.VersionHandler(schemas, "v5")

	//framework route
	r.Methods(http.MethodGet).Path("/").Handler(versionsHandler)
	r.Methods(http.MethodGet).Path("/v5").Handler(versionHandler)
	r.Methods(http.MethodGet).Path("/v5/apiversions").Handler(versionsHandler)
	r.Methods(http.MethodGet).Path("/v5/schemas").Handler(api.SchemasHandler(schemas))
	r.Methods(http.MethodGet).Path("/v5/schemas/{id}").Handler(api.SchemaHandler(schemas))

	//notifier route
	r.Methods(http.MethodGet).Path("/v5/notifiers").Handler(f(schemas, s.notifiersList))
	r.Methods(http.MethodGet).Path("/v5/notifier").Handler(f(schemas, s.notifiersList))
	r.Methods(http.MethodPost).Path("/v5/notifiers").Handler(f(schemas, s.createNotifier))
	r.Methods(http.MethodPost).Path("/v5/notifier").Handler(f(schemas, s.createNotifier))
	r.Methods(http.MethodGet).Path("/v5/notifiers/{id}").Handler(f(schemas, s.getNotifier))
	r.Methods(http.MethodDelete).Path("/v5/notifiers/{id}").Handler(f(schemas, s.deleteNotifier))
	r.Methods(http.MethodPut).Path("/v5/notifiers/{id}").Handler(f(schemas, s.updateNotifier))

	//recipient route
	r.Methods(http.MethodGet).Path("/v5/recipient").Handler(f(schemas, s.recipientsList))
	r.Methods(http.MethodGet).Path("/v5/recipients").Handler(f(schemas, s.recipientsList))
	r.Methods(http.MethodPost).Path("/v5/recipients").Handler(f(schemas, s.createRecipient))
	r.Methods(http.MethodPost).Path("/v5/recipients").Handler(f(schemas, s.createRecipient))
	r.Methods(http.MethodGet).Path("/v5/recipients/{id}").Handler(f(schemas, s.getRecipient))
	r.Methods(http.MethodDelete).Path("/v5/recipients/{id}").Handler(f(schemas, s.deleteRecipient))
	r.Methods(http.MethodPut).Path("/v5/recipients/{id}").Handler(f(schemas, s.updateRecipient))

	//alert route
	r.Methods(http.MethodGet).Path("/v5/alert").Handler(f(schemas, s.alertsList))
	r.Methods(http.MethodGet).Path("/v5/alerts").Handler(f(schemas, s.alertsList))
	r.Methods(http.MethodPost).Path("/v5/alert").Handler(f(schemas, s.createAlert))
	r.Methods(http.MethodPost).Path("/v5/alerts").Handler(f(schemas, s.createAlert))
	r.Methods(http.MethodGet).Path("/v5/alerts/{id}").Handler(f(schemas, s.getAlert))
	r.Methods(http.MethodDelete).Path("/v5/alerts/{id}").Handler(f(schemas, s.deleteAlert))
	r.Methods(http.MethodPut).Path("/v5/alerts/{id}").Handler(f(schemas, s.updateAlert))

	return r
}
