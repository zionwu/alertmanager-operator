package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/handlers"
	"github.com/urfave/cli"
	"github.com/zionwu/alertmanager-operator/alertmanager"
	"github.com/zionwu/alertmanager-operator/api"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var VERSION = "0.0.1"

var (
	cfg api.Config
)

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
}

func RunOperator(c *cli.Context) error {

	if c.Bool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	cfg = api.Config{}

	kubeconfig := c.String("kubeconfig")
	listenPort := c.String("listen-port")
	cfg.ManagerUrl = c.String("alertmanager-url")
	cfg.SecretName = c.String("alertmanager-secret-name")

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

	router := http.Handler(api.NewRouter(api.NewServer(config)))
	router = handlers.LoggingHandler(os.Stdout, router)
	router = handlers.ProxyHeaders(router)
	logrus.Infof("Alertmanager operator running on %s", listenPort)
	go http.ListenAndServe(":"+listenPort, router)

	alertmanagerOperator, err := alertmanager.NewOperator(config, &cfg)
	if err != nil {
		panic(err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	wg, ctx := errgroup.WithContext(ctx)

	wg.Go(func() error { return alertmanagerOperator.Run(ctx.Done()) })

	term := make(chan os.Signal)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	select {
	case <-term:
		logrus.Info("msg", "Received SIGTERM, exiting gracefully...")
	case <-ctx.Done():
	}

	cancel()
	if err := wg.Wait(); err != nil {
		logrus.Errorf("msg", "Unhandled error received. Exiting: %v", err)
		return err
	}

	return nil
}
