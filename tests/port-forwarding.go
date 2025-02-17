package tests

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type PortForwarder interface {
	Connect(pod *core.Pod, remotePort uint16) (net.Conn, error)
}

type portForwarderImpl struct {
	config    *rest.Config
	client    rest.Interface
	requestId int32
}

var _ PortForwarder = &portForwarderImpl{}

type portForwardConn struct {
	net.Conn
	streamConnCloser io.Closer
}

func (p *portForwardConn) Close() error {
	return errors.Join(p.Conn.Close(), p.streamConnCloser.Close())
}

func (p *portForwarderImpl) Connect(pod *core.Pod, remotePort uint16) (net.Conn, error) {
	streamConnection, err := p.createStreamConnection(pod)
	if err != nil {
		return nil, err
	}

	requestId := atomic.AddInt32(&p.requestId, 1)

	// Error stream is needed, otherwise port-forwarding will not work
	headers := http.Header{}
	headers.Set(core.StreamType, core.StreamTypeError)
	headers.Set(core.PortHeader, fmt.Sprintf("%d", remotePort))
	headers.Set(core.PortForwardRequestIDHeader, strconv.Itoa(int(requestId)))
	errorStream, err := streamConnection.CreateStream(headers)
	if err != nil {
		streamConnection.Close()
		return nil, err
	}
	// We will not write to error stream
	errorStream.Close()

	headers.Set(core.StreamType, core.StreamTypeData)
	dataStream, err := streamConnection.CreateStream(headers)
	if err != nil {
		streamConnection.Close()
		return nil, err
	}

	return &portForwardConn{
		Conn:             dataStream.(net.Conn),
		streamConnCloser: streamConnection,
	}, nil
}

func (p *portForwarderImpl) createStreamConnection(pod *core.Pod) (httpstream.Connection, error) {
	transport, upgrader, err := spdy.RoundTripperFor(p.config)
	if err != nil {
		return nil, err
	}

	req := p.client.Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("portforward")

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	streamConn, _, err := dialer.Dial(portforward.PortForwardProtocolV1Name)
	return streamConn, err
}

func NewPortForwarder(config *rest.Config, client rest.Interface) PortForwarder {
	return &portForwarderImpl{
		config: config,
		client: client,
	}
}
