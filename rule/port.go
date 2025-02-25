package rules

import (
	"fmt"
	"strconv"
	"strings"

	C "github.com/Dreamacro/clash/constant"
)

type PortType int

const (
	PortTypeSrc PortType = iota
	PortTypeDest
	PortTypeInbound
)

type portReal struct {
	portStart int
	portEnd   int
}

type Port struct {
	*Base
	adapter  string
	port     string
	portType PortType
	portList []portReal
}

func (p *Port) RuleType() C.RuleType {
	switch p.portType {
	case PortTypeSrc:
		return C.SrcPort
	case PortTypeDest:
		return C.DstPort
	case PortTypeInbound:
		return C.InboundPort
	default:
		panic(fmt.Errorf("unknown port type: %v", p.portType))
	}
}

func (p *Port) Match(metadata *C.Metadata) bool {
	switch p.portType {
	case PortTypeSrc:
		return p.matchPortReal(int(metadata.SrcPort))
	case PortTypeDest:
		return p.matchPortReal(int(metadata.DstPort))
	case PortTypeInbound:
		return p.matchPortReal(int(metadata.OriginDst.Port()))
	default:
		panic(fmt.Errorf("unknown port type: %v", p.portType))
	}
}

func (p *Port) Adapter() string {
	return p.adapter
}

func (p *Port) Payload() string {
	return p.port
}

func (p *Port) ShouldResolveIP() bool {
	return false
}

func (p *Port) matchPortReal(port int) bool {
	var rs bool
	for _, pr := range p.portList {
		if pr.portEnd == -1 {
			rs = port == pr.portStart
		} else {
			rs = port >= pr.portStart && port <= pr.portEnd
		}
		if rs {
			return true
		}
	}
	return false
}

func NewPort(port string, adapter string, portType PortType) (*Port, error) {
	ports := strings.Split(port, "/")
	if len(ports) > 28 {
		return nil, fmt.Errorf("%s, too many ports to use, maximum support 28 ports", errPayload.Error())
	}

	var portList []portReal
	for _, p := range ports {
		if p == "" {
			continue
		}

		subPorts := strings.Split(p, "-")
		subPortsLen := len(subPorts)
		if subPortsLen > 2 {
			return nil, errPayload
		}

		portStart, err := strconv.ParseUint(strings.Trim(subPorts[0], "[ ]"), 10, 16)
		if err != nil {
			return nil, errPayload
		}

		switch subPortsLen {
		case 1:
			portList = append(portList, portReal{int(portStart), -1})
		case 2:
			portEnd, err := strconv.ParseUint(strings.Trim(subPorts[1], "[ ]"), 10, 16)
			if err != nil {
				return nil, errPayload
			}

			shouldReverse := portStart > portEnd
			if shouldReverse {
				portList = append(portList, portReal{int(portEnd), int(portStart)})
			} else {
				portList = append(portList, portReal{int(portStart), int(portEnd)})
			}
		}
	}

	if len(portList) == 0 {
		return nil, errPayload
	}

	return &Port{
		Base:     &Base{},
		adapter:  adapter,
		port:     port,
		portType: portType,
		portList: portList,
	}, nil
}

var _ C.Rule = (*Port)(nil)
