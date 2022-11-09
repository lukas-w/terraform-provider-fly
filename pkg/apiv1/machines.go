package apiv1

import (
	"errors"
	"fmt"
	"github.com/Khan/genqlient/graphql"
	hreq "github.com/imroc/req/v3"
	"github.com/samber/lo"
	"github.com/superfly/flyctl/api"
	"net/http"
	"time"
)

var NonceHeader = "fly-machine-lease-nonce"

type MachineAPI struct {
	client     *graphql.Client
	httpClient *hreq.Client
	endpoint   string
}

type CpuType string

const (
	CpuTypeShared    CpuType = "shared"
	CpuTypeDedicated CpuType = "dedicated"
)

type Machines []*api.Machine

func (m Machines) WithoutInactive() Machines {
	return lo.Filter(m, func(m *api.Machine, _ int) bool {
		return m.Config != nil && m.Config.Metadata["process_group"] != "release_command" && m.State != "destroyed"
	})
}

func NewMachineAPI(httpClient *hreq.Client, endpoint string) *MachineAPI {
	return &MachineAPI{
		httpClient: httpClient,
		endpoint:   endpoint,
	}
}

func (a *MachineAPI) LockMachine(app string, id string, timeout int) (*api.MachineLease, error) {
	var res api.MachineLease
	_, err := a.httpClient.R().SetResult(&res).Post(fmt.Sprintf("http://%s/v1/apps/%s/machines/%s/lease/?ttl=%d", a.endpoint, app, id, timeout))
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (a *MachineAPI) ReleaseMachine(lease api.MachineLease, app string, id string) error {
	_, err := a.httpClient.R().SetHeader(NonceHeader, lease.Data.Nonce).Delete(fmt.Sprintf("http://%s/v1/apps/%s/machines/%s/lease", a.endpoint, app, id))
	if err != nil {
		return err
	}
	return nil
}

func (a *MachineAPI) WaitForMachine(app string, id string, instanceID string) error {
	_, err := a.httpClient.R().Get(fmt.Sprintf("http://%s/v1/apps/%s/machines/%s/wait?instance_id=%s", a.endpoint, app, id, instanceID))
	return err
}

// CreateMachine takes a MachineCreateOrUpdateRequest and creates the requested machine in the given app and then writes the response into the `res` param
func (a *MachineAPI) CreateMachine(input api.LaunchMachineInput, app string, res *api.Machine) error {
	createResponse, err := a.httpClient.R().SetBody(input).SetResult(res).Post(fmt.Sprintf("http://%s/v1/apps/%s/machines", a.endpoint, app))

	if err != nil {
		return err
	}

	if createResponse.StatusCode != http.StatusCreated && createResponse.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("Create request failed: %s, %+v", createResponse.Status, createResponse.String()))
	}
	return nil
}

func (a *MachineAPI) UpdateMachine(input api.LaunchMachineInput, app string, id string, res *api.Machine) error {
	lease, err := a.LockMachine(app, id, 30)
	if err != nil {
		return err
	}
	reqRes, err := a.httpClient.R().SetBody(input).SetResult(res).SetHeader(NonceHeader, lease.Data.Nonce).Post(fmt.Sprintf("http://%s/v1/apps/%s/machines/%s", a.endpoint, app, id))
	if err != nil {
		return err
	}
	err = a.ReleaseMachine(*lease, app, id)
	if err != nil {
		return err
	}
	if reqRes.StatusCode != http.StatusCreated && reqRes.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("Update request failed: %s, %+v", reqRes.Status, reqRes.String()))
	}
	return nil
}

func (a *MachineAPI) ReadMachine(app string, id string, res *api.Machine) (*hreq.Response, error) {
	return a.httpClient.R().SetResult(res).Get(fmt.Sprintf("http://%s/v1/apps/%s/machines/%s", a.endpoint, app, id))
}

func (a *MachineAPI) DeleteMachine(app string, id string, maxRetries int) error {
	deleted := false
	for i := 0; i < maxRetries; i++ {
		var machine api.Machine
		readResponse, err := a.httpClient.R().SetResult(&machine).Get(fmt.Sprintf("http://%s/v1/apps/%s/machines/%s", a.endpoint, app, id))
		if err != nil {
			return err
		}

		if readResponse.StatusCode == 200 {
			if machine.State == "started" || machine.State == "starting" || machine.State == "replacing" {
				_, _ = a.httpClient.R().Post(fmt.Sprintf("http://%s/v1/apps/%s/machines/%s/stop", a.endpoint, app, id))
			}
			if machine.State == "stopping" || machine.State == "destroying" {
				time.Sleep(5 * time.Second)
			}
			if machine.State == "stopped" || machine.State == "replaced" {
				_, err = a.httpClient.R().Delete(fmt.Sprintf("http://%s/v1/apps/%s/machines/%s", a.endpoint, app, id))
				if err != nil {
					return err
				}
			}
			if machine.State == "destroyed" {
				deleted = true
				break
			}
		}
	}
	if !deleted {
		return errors.New("max retries exceeded")
	}
	return nil
}

func (a *MachineAPI) ListMachines(app string, res *Machines) (*hreq.Response, error) {
	*res = make(Machines, 0)
	return a.httpClient.R().SetResult(res).Get(fmt.Sprintf("http://%s/v1/apps/%s/machines", a.endpoint, app))
}
