package main

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/handlers"
	"github.com/urfave/cli"
	"github.com/zionwu/alertmanager-operator/api"
	"github.com/zionwu/alertmanager-operator/client/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var VERSION = "0.0.1"

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})

	app := cli.NewApp()
	app.Version = VERSION
	app.Usage = "AlertManager Operator"
	app.Action = RunOperator

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug, d",
			Usage:  "enable debug logging level",
			EnvVar: "RANCHER_DEBUG",
		},
		//TODO: change the default value to "" so that it will use service account
		cli.StringFlag{
			Name:   "kubeconfig, k",
			Usage:  "(optional) absolute path to the kubeconfig file",
			EnvVar: "KUBECONFIG",
			Value:  filepath.Join("/Users/wuziyang/", ".kube", "config"),
		},
		cli.StringFlag{
			Name:   "listen-port, l",
			Usage:  "server listening port",
			EnvVar: "LISTEN_PORT",
			Value:  "8888",
		},
		cli.StringFlag{
			Name:   "alertmanager-url, u",
			Usage:  "AlertManager access URL",
			EnvVar: "ALERTMANAGER_URL",
			Value:  "http://192.168.99.100:31285",
		},
		//TODO: support using config file from local path
		cli.StringFlag{
			Name:   "alertmanager-config-file, f",
			Usage:  "AlertManager config file location, if it is not empty, operator will first try to use local config file",
			EnvVar: "ALERTMANAGER_CONFIG_FILE",
			Value:  "alertmanager-config-file",
		},
		cli.StringFlag{
			Name:   "alertmanager-secret-name, s",
			Usage:  "AlertManager secret name",
			EnvVar: "ALERTMANAGER_SECRET_NAME",
			Value:  "alertmanager-config2",
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatalf("Critical error: %v", err)
	}
	logrus.Info("Alertmanager operator started")

}

func RunOperator(c *cli.Context) error {

	if c.Bool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	kubeconfig := c.String("kubeconfig")
	listenPort := c.String("listen-port")
	alertmanagerURL := c.String("alertmanager-url")
	alertmanagerSecretName := c.String("alertmanager-secret-name")
	alertmanagerConfig := c.String("alertmanager-config-file")

	var config *rest.Config
	var err error
	if kubeconfig == "" {
		if config, err = rest.InClusterConfig(); err != nil {
			panic(err.Error())
		}

	} else {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err.Error())
		}
	}

	if err != nil {
		panic(err.Error())
	}
	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	mclient, err := v1beta1.NewForConfig(config)

	router := http.Handler(api.NewRouter(api.NewServer(clientset, mclient, alertmanagerURL, alertmanagerSecretName, alertmanagerConfig)))

	router = handlers.LoggingHandler(os.Stdout, router)
	router = handlers.ProxyHeaders(router)

	logrus.Infof("Alertmanager operator running on %s", listenPort)

	return http.ListenAndServe(":"+listenPort, router)
}
