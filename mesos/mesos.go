package mesos

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/CiscoCloud/mesos-consul/config"
	"github.com/CiscoCloud/mesos-consul/consul"
	"github.com/CiscoCloud/mesos-consul/registry"
	"github.com/CiscoCloud/mesos-consul/state"

	consulapi "github.com/hashicorp/consul/api"
	proto "github.com/mesos/mesos-go/mesosproto"
	log "github.com/sirupsen/logrus"
)

type CacheEntry struct {
	service      *consulapi.AgentServiceRegistration
	isRegistered bool
}

type Mesos struct {
	Registry registry.Registry
	Agents   map[string]string
	Lock     sync.Mutex

	Leader  *proto.MasterInfo
	Masters []*proto.MasterInfo
	started sync.Once
	startChan chan struct{}
}

func New(c *config.Config) *Mesos {
	m := new(Mesos)

	if c.Zk == "" {
		return nil
	}

	if consul.IsEnabled() {
		m.Registry = consul.New()
	}

	if m.Registry == nil {
		log.Fatal("No registry specified")
	}

	m.zkDetector(c.Zk)

	return m
}

func (m *Mesos) Refresh() error {
	sj, err := m.loadState()
	if err != nil {
		log.Warn("loadState failed: ", err.Error())
		return err
	}

	if sj.Leader == "" {
		return errors.New("Empty master")
	}

	if m.Registry.CacheCreate() {
		m.LoadCache()
	}

	m.parseState(sj)

	return nil
}

func (m *Mesos) loadState() (state.State, error) {
	var err error
	var sj state.State

	log.Debug("loadState() called")

	defer func() {
		if rec := recover(); rec != nil {
			err = errors.New("can't connect to Mesos")
		}
	}()

	mh := m.getLeader()
	if mh.Ip == "" {
		log.Warn("No master in zookeeper")
		return sj, errors.New("No master in zookeeper")
	}

	log.Infof("Zookeeper leader: %s:%s", mh.Ip, mh.PortString)

	log.Info("reloading from master ", mh.Ip)
	sj = m.loadFromMaster(mh.Ip, mh.PortString)

	if rip := leaderIP(sj.Leader); rip != mh.Ip {
		log.Warn("master changed to ", rip)
		sj = m.loadFromMaster(rip, mh.PortString)
	}

	return sj, err
}

func (m *Mesos) loadFromMaster(ip string, port string) (sj state.State) {
	url := "http://" + ip + ":" + port + "/master/state.json"

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(body, &sj)
	if err != nil {
		log.Fatal(err)
	}

	return sj
}

func (m *Mesos) parseState(sj state.State) {
	log.Info("Running parseState")

	m.RegisterHosts(sj)
	log.Debug("Done running RegisterHosts")

	for _, fw := range sj.Frameworks {
		for _, task := range fw.Tasks {
			agent, ok := m.Agents[task.SlaveID]
			if ok && task.State == "TASK_RUNNING" {
				m.registerTask(&task, agent)
			}
		}
	}

	// Remove completed tasks
	m.Registry.Deregister()
}
