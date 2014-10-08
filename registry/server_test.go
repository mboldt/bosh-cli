package registry_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-agent/logger"

	. "github.com/cloudfoundry/bosh-micro-cli/registry"
)

var _ = Describe("Server", func() {
	var (
		server                   Server
		registryURL              string
		incorrectAuthRegistryURL string
		client                   helperClient
	)

	BeforeEach(func() {
		registryHost := "localhost:6901"
		registryURL = fmt.Sprintf("http://fake-user:fake-password@%s", registryHost)
		incorrectAuthRegistryURL = fmt.Sprintf("http://incorrect-user:incorrect-password@%s", registryHost)
		logger := boshlog.NewLogger(boshlog.LevelNone)
		server = NewServer("fake-user", "fake-password", "localhost", 6901, logger)
		go server.Start()
		client.WaitForEndpoint("http://"+registryHost, 1*time.Second)
		httpClient := http.Client{}
		client = NewHelperClient(httpClient)
	})

	AfterEach(func() {
		server.Stop()
	})

	Describe("making a request with an unknown path", func() {
		It("returns 404", func() {
			_, _, statusCode := client.DoPut(registryURL+"/instances/1/something-else", "fake-agent-settings")
			Expect(statusCode).To(Equal(404))
		})
	})

	Describe("PUT instances/:instance_id/settings", func() {
		Context("when username and password are incorrect", func() {
			It("returns 401", func() {
				_, responseHeader, statusCode := client.DoPut(incorrectAuthRegistryURL+"/instances/1/settings", "fake-agent-settings")
				Expect(statusCode).To(Equal(401))
				Expect(responseHeader.Get("WWW-Authenticate")).To(Equal(`Basic realm="Bosh Registry"`))
			})
		})

		Context("when the settings do not yet exist", func() {
			It("creates the settings", func() {
				_, _, statusCode := client.DoPut(registryURL+"/instances/1/settings", "fake-agent-settings")
				Expect(statusCode).To(Equal(201))

				httpBody, statusCode := client.DoGet(registryURL + "/instances/1/settings")
				Expect(statusCode).To(Equal(200))
				Expect(httpBody).To(Equal("fake-agent-settings"))
			})
		})

		Context("when the settings already exist", func() {
			It("updates the settings", func() {
				_, _, statusCode := client.DoPut(registryURL+"/instances/1/settings", "fake-agent-settings")
				Expect(statusCode).To(Equal(201))

				_, _, statusCode = client.DoPut(registryURL+"/instances/1/settings", "fake-agent-settings-updated")
				Expect(statusCode).To(Equal(200))

				httpBody, statusCode := client.DoGet(registryURL + "/instances/1/settings")
				Expect(statusCode).To(Equal(200))
				Expect(httpBody).To(Equal("fake-agent-settings-updated"))
			})
		})
	})

	Describe("DELETE instances/:instance_id/settings", func() {
		Context("when username and password are incorrect", func() {
			It("returns 401", func() {
				responseHeader, statusCode := client.DoDelete(incorrectAuthRegistryURL + "/instances/1/settings")
				Expect(statusCode).To(Equal(401))
				Expect(responseHeader.Get("WWW-Authenticate")).To(Equal(`Basic realm="Bosh Registry"`))
			})
		})

		Context("when the settings exist", func() {
			It("deletes the settings", func() {
				_, _, statusCode := client.DoPut(registryURL+"/instances/1/settings", "fake-agent-settings")
				Expect(statusCode).To(Equal(201))

				_, statusCode = client.DoDelete(registryURL + "/instances/1/settings")
				Expect(statusCode).To(Equal(200))

				_, statusCode = client.DoGet(registryURL + "/instances/1/settings")
				Expect(statusCode).To(Equal(404))
			})
		})

		Context("when the settings do not exist", func() {
			It("deletes the settings", func() {
				_, statusCode := client.DoDelete(registryURL + "/instances/1/settings")
				Expect(statusCode).To(Equal(200))

				_, statusCode = client.DoGet(registryURL + "/instances/1/settings")
				Expect(statusCode).To(Equal(404))
			})
		})
	})

	Describe("GET instances/:instance_id/settings", func() {
		Context("when settings do not exist", func() {
			It("returns 404", func() {
				_, statusCode := client.DoGet(registryURL + "/instances/1/settings")
				Expect(statusCode).To(Equal(404))
			})
		})

		Context("when settings exist", func() {
			BeforeEach(func() {
				_, _, statusCode := client.DoPut(registryURL+"/instances/1/settings", "fake-agent-settings")
				Expect(statusCode).To(Equal(201))
			})

			Context("when username and password are incorrect", func() {
				It("returns 200", func() {
					httpBody, statusCode := client.DoGet(incorrectAuthRegistryURL + "/instances/1/settings")
					Expect(statusCode).To(Equal(200))
					Expect(httpBody).To(Equal("fake-agent-settings"))
				})
			})
		})
	})
})

type helperClient struct {
	httpClient http.Client
}

func NewHelperClient(httpClient http.Client) helperClient {
	return helperClient{
		httpClient: httpClient,
	}
}

func (c helperClient) DoDelete(endpoint string) (http.Header, int) {
	request, err := http.NewRequest("DELETE", endpoint, strings.NewReader(""))
	Expect(err).ToNot(HaveOccurred())
	httpResponse, err := c.httpClient.Do(request)
	Expect(err).ToNot(HaveOccurred())

	return httpResponse.Header, httpResponse.StatusCode
}

func (c helperClient) DoPut(endpoint string, body string) (string, http.Header, int) {
	putPayload := strings.NewReader(body)

	request, err := http.NewRequest("PUT", endpoint, putPayload)
	Expect(err).ToNot(HaveOccurred())

	httpResponse, err := c.httpClient.Do(request)
	Expect(err).ToNot(HaveOccurred())

	defer httpResponse.Body.Close()

	httpBody, err := ioutil.ReadAll(httpResponse.Body)
	Expect(err).ToNot(HaveOccurred())

	return string(httpBody), httpResponse.Header, httpResponse.StatusCode
}

func (c helperClient) DoGet(endpoint string) (string, int) {
	httpResponse, err := c.httpClient.Get(endpoint)
	Expect(err).ToNot(HaveOccurred())

	httpBody, err := ioutil.ReadAll(httpResponse.Body)
	Expect(err).ToNot(HaveOccurred())

	return string(httpBody), httpResponse.StatusCode
}

func (c helperClient) WaitForEndpoint(endpoint string, timeout time.Duration) {
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); {
		_, err := c.httpClient.Get(endpoint)
		if err == nil {
			return
		}
	}
}