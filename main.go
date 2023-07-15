package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	masterAddr *net.TCPAddr
	saddr      *net.TCPAddr

	apiAddr      = flag.String("kubeapi", "kubernetes.default.svc:443", "kubeapi address")
	sentinelAddr = flag.String("sentinel", "127.0.0.1:26379", "sentinel address")
	masterName   = flag.String("master", "mymaster", "name of the master redis node")
	serviceName  = flag.String("service", "", "name of serive endpoint to configure")
	logLevel     = flag.String("loglevel", "info", "Log level")
)

func main() {
	flag.Parse()
	lvl, _ := log.ParseLevel(*logLevel)
	log.SetLevel(lvl)

	_, err := net.ResolveTCPAddr("tcp", *apiAddr)
	if err != nil {
		log.Fatalf("Failed to resolve api address: %s", err)
	}
	saddr, err = net.ResolveTCPAddr("tcp", *sentinelAddr)
	if err != nil {
		log.Fatalf("Failed to resolve sentinel address: %s", err)
	}
	if len(*serviceName) == 0 {
		log.Fatal("Service name must be set")
	}
	i := 0
	for {
		currentMasterAddr, err := getMasterAddr(saddr, *masterName)
		if err != nil {
			log.Error(err)
		} else {
			if masterAddr == nil || masterAddr.Port != currentMasterAddr.Port || strings.Compare(masterAddr.IP.String(), currentMasterAddr.IP.String()) != 0 {
				err = changeEndpoint(currentMasterAddr)
				if err == nil {
					masterAddr = currentMasterAddr
					log.Warnf("Master endpoint changed to %s", masterAddr.IP.String())
					i = 0
				} else {
					log.Error(err)
				}
			} else {
				i = i + 1
				if i >= 15 {
					err = changeEndpoint(currentMasterAddr)
					if err == nil {
						log.Infof("Synced endpoint to %s", masterAddr.IP.String())
						i = 0
					} else {
						log.Errorf("Can't sync endpoint to %s", masterAddr.IP.String())
					}
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
}

func changeEndpoint(currentMasterAddr *net.TCPAddr) error {
	token, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return err
	}
	namespace, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return err
	}
	// Load CA cert
	caCert, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		return err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	// Setup HTTPS client
	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}

	data := fmt.Sprintf("[{\"op\": \"replace\", \"path\": \"/subsets\", \"value\": [{ \"addresses\": [{\"ip\": \"%s\"}],\"ports\": [{\"name\": \"redis\",\"port\": %d,\"protocol\": \"TCP\"}]}]}]", currentMasterAddr.IP.String(), currentMasterAddr.Port)
	log.Debug(data)

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{
		Transport: transport,
		Timeout:   2 * time.Second,
	}

	url := fmt.Sprintf("https://%s/api/v1/namespaces/%s/endpoints/%s", *apiAddr, namespace, *serviceName)
	log.Debug(url)
	req, err := http.NewRequest(http.MethodPatch, url, strings.NewReader(data))
	if err != nil {
		return err
	}
	defer req.Body.Close()
	req.Header.Add("Content-Type", "application/json-patch+json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	_, err = client.Do(req)
	return err
}

func getMasterAddr(sentinelAddress *net.TCPAddr, masterName string) (*net.TCPAddr, error) {
	conn, err := net.DialTCP("tcp", nil, sentinelAddress)
	if err != nil {
		return nil, err
	}

	defer conn.Close()

	conn.Write([]byte(fmt.Sprintf("sentinel get-master-addr-by-name %s\n", masterName)))

	b := make([]byte, 256)
	_, err = conn.Read(b)
	if err != nil {
		log.Fatal(err)
	}

	parts := strings.Split(string(b), "\r\n")

	if len(parts) < 5 {
		err = errors.New("couldn't get master address from sentinel")
		return nil, err
	}

	if parts[2] == "127.0.0.1" {
		err = errors.New("got 127.0.0.1 from sentinel, skip response")
		return nil, err
	}

	//getting the string address for the master node
	stringaddr := fmt.Sprintf("%s:%s", parts[2], parts[4])
	addr, err := net.ResolveTCPAddr("tcp", stringaddr)

	if err != nil {
		return nil, err
	}

	return addr, err
}
