package vmm

import (
	"errors"
	"fmt"
)

var (
	ErrUnsupportedRequest = errors.New("api: unsupported request")
)

// VmmPingResponse reports that the VMM API is available.
type VmmPingResponse struct {
	Version string `json:"version"`
}

// VmInfoResponse describes the current VM without exposing mutable internals.
type VmInfoResponse struct {
	State            VmState  `json:"state"`
	MemoryActualSize uint64   `json:"memory_actual_size"`
	Config           VmConfig `json:"config"`
}

// APIRequest is the minimal in-process lifecycle API. An HTTP transport can
// be layered on top without changing VMM semantics.
type APIRequest struct {
	Type   string    `json:"type"`
	Config *VmConfig `json:"config,omitempty"`
}

// APIResponse contains exactly one of the supported response payloads.
type APIResponse struct {
	Ping *VmmPingResponse `json:"ping,omitempty"`
	Info *VmInfoResponse  `json:"info,omitempty"`
}

// HandleRequest executes a lifecycle request against this VMM.
func (v *VMM) HandleRequest(request APIRequest) (APIResponse, error) {
	switch request.Type {
	case "vmm.ping":
		return APIResponse{Ping: &VmmPingResponse{Version: "0.0.1"}}, nil
	case "vm.create":
		if request.Config == nil {
			return APIResponse{}, ErrVmMissingConfig
		}
		if _, err := v.CreateVM(*request.Config); err != nil {
			return APIResponse{}, err
		}
		return APIResponse{}, nil
	case "vm.boot":
		vm := v.VM()
		if vm == nil {
			return APIResponse{}, ErrNoVM
		}
		return APIResponse{}, vm.Boot()
	case "vm.pause":
		vm := v.VM()
		if vm == nil {
			return APIResponse{}, ErrNoVM
		}
		return APIResponse{}, vm.Pause()
	case "vm.shutdown", "vm.delete":
		return APIResponse{}, v.Shutdown()
	case "vm.info":
		vm := v.VM()
		if vm == nil {
			return APIResponse{}, ErrNoVM
		}
		return APIResponse{Info: &VmInfoResponse{
			State:            vm.State(),
			MemoryActualSize: vm.memory.Size(),
			Config:           vm.Config(),
		}}, nil
	default:
		return APIResponse{}, fmt.Errorf("%w: %q", ErrUnsupportedRequest, request.Type)
	}
}
