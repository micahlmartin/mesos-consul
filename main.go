package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/CiscoCloud/mesos-consul/config"
	"github.com/CiscoCloud/mesos-consul/consul"
	"github.com/CiscoCloud/mesos-consul/mesos"

	flag "github.com/ogier/pflag"
	log "github.com/sirupsen/logrus"
)

const Name = "mesos-consul"
const Version = "0.3"

func main() {
	c, err := parseFlags(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Using registry port: ", c.RegistryPort)
	log.Info("Using zookeeper: ", c.Zk)
	leader := mesos.New(c)

	ticker := time.NewTicker(c.Refresh)
	leader.Refresh()
	for _ = range ticker.C {
		leader.Refresh()
	}
}

func parseFlags(args []string) (*config.Config, error) {
	var doHelp bool
	var c = config.DefaultConfig()

	flags := flag.NewFlagSet("mesos-consul", flag.ContinueOnError)
	flags.Usage = func() {
		fmt.Println(Help())
	}

	flags.BoolVar(&doHelp, "help", false, "")
	flags.StringVar(&c.LogLevel, "log-level", "WARN", "")
	flags.DurationVar(&c.Refresh, "refresh", time.Minute, "")
	flags.StringVar(&c.RegistryPort, "registry-port", "8500", "")
	flags.Var((*config.AuthVar)(c.RegistryAuth), "registry-auth", "")
	flags.BoolVar(&c.RegistrySSL.Enabled, "registry-ssl", c.RegistrySSL.Enabled, "")
	flags.BoolVar(&c.RegistrySSL.Verify, "registry-ssl-verify", c.RegistrySSL.Verify, "")
	flags.StringVar(&c.RegistrySSL.Cert, "registry-ssl-cert", c.RegistrySSL.Cert, "")
	flags.StringVar(&c.RegistrySSL.CaCert, "registry-ssl-cacert", c.RegistrySSL.CaCert, "")
	flags.StringVar(&c.RegistryToken, "registry-token", c.RegistryToken, "")
	flags.StringVar(&c.Zk, "zk", "zk://127.0.0.1:2181/mesos", "")

	consul.AddCmdFlags(flags)

	if err := flags.Parse(args); err != nil {
		return nil, err
	}

	args = flags.Args()
	if len(args) > 0 {
		return nil, fmt.Errorf("extra argument(s): %q", args)
	}

	if doHelp {
		flags.Usage()
		os.Exit(0)
	}

	l, err := log.ParseLevel(c.LogLevel)
	if err != nil {
		log.SetLevel(log.WarnLevel)
		log.Warnf("Invalid log level '%v'. Setting to WARN")
	} else {
		log.SetLevel(l)
	}

	return c, nil
}

func Help() string {
	helpText := `
Usage: mesos-consul [options]

Options:

  --log-level=<log_level>	Set the Logging level to one of [ "DEBUG", "INFO", "WARN", "ERROR" ]
				(default "WARN")
  --refresh=<time>		Set the Mesos refresh rate
				(default 1m)
  --registry-auth=<user[:pass]>	Set the basic authentication username
				(and password)
  --registry-port=<port>	Port to connect to consul agents
				(default 8500)
  --registry-ssl		Use SSL when connecting to the registry
  --registry-ssl-verify		Verify certificates when connecting via SSL
  --registry-ssl-cert		SSL certificates to send to registry
  --registry-ssl-cacert		Validate server certificate against this CA
				certificate file list
  --registry-token=<token>	Set registry ACL token
  --zk=<address>		Zookeeper path to Mesos
				(default zk://127.0.0.1:2181/mesos)
` + consul.Help()

	return strings.TrimSpace(helpText)
}
