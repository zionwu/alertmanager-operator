package main

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/handlers"
	"github.com/urfave/cli"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/zionwu/alertmanager-operator/api"
	"github.com/zionwu/alertmanager-operator/client/v1beta1"
)

const (
	sockFile = "/var/run/longhorn/volume-manager.sock"

	FlagEngineImage  = "engine-image"
	FlagOrchestrator = "orchestrator"
	FlagETCDServers  = "etcd-servers"
	FlagETCDPrefix   = "etcd-prefix"

	FlagDockerNetwork = "docker-network"

	EnvEngineImage = "LONGHORN_ENGINE_IMAGE"
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
		cli.StringFlag{
			Name:   "kubeconfig, k",
			Usage:  "(optional) absolute path to the kubeconfig file",
			EnvVar: "KUBECONFIG",
			Value:  filepath.Join("/Users/wuziyang/", ".kube", "config"),
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatalf("Critical error: %v", err)
	}

}

func RunOperator(c *cli.Context) error {

	if c.Bool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	kubeconfig := c.String("kubeconfig")
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)

	mclient, err := v1beta1.NewForConfig(config)

	router := http.Handler(api.NewRouter(api.NewServer(clientset, mclient)))

	router = handlers.LoggingHandler(os.Stdout, router)
	router = handlers.ProxyHeaders(router)

	return http.ListenAndServe(":8888", router)
}
