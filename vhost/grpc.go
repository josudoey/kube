package vhost

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

const (
	// http2MaxFrameLen specifies the max length of a HTTP2 frame.
	http2MaxFrameLen = 16384 // 16KB frame
	// http://http2.github.io/http2-spec/#SettingValues
	http2InitHeaderTableSize = 4096
	// baseContentType is the base content-type for gRPC.  This is a valid
	// content-type on it's own, but can also include a content-subtype such as
	// "proto" as a suffix after "+" or ";".  See
	// https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md#requests
	// for more details.

	defaultServerMaxHeaderListSize = uint32(16 << 20)
)

type readRecorder struct {
	io.Reader
	bytes.Buffer
}

func (r *readRecorder) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.Buffer.Write(p[0:n])
	return n, err
}

type writeRecorder struct {
	io.Writer
	bytes.Buffer
}

func (r *writeRecorder) Write(p []byte) (int, error) {
	r.Buffer.Write(p)
	return r.Writer.Write(p)
}

type GRPCPreface struct {
	Header        []hpack.HeaderField
	ServerPreface []byte
	ClientPreface []byte
}

var ErrInlvaidGRPCPreface = errors.New("invalid grpc preface data")

// see https://github.com/grpc/grpc-go/blob/01bababd83492b6eb1c7046ab4c3a4b1bcc5e9d6/internal/transport/http2_server.go#L135
func GetGRPCPreface(conn net.Conn, rw *bufio.ReadWriter) (*GRPCPreface, error) {
	const http2ClientPrefaceSuffix = "SM\r\n\r\n"
	if _, err := io.ReadFull(rw, make([]byte, len(http2ClientPrefaceSuffix))); err != nil {
		return nil, err
	}

	r := &readRecorder{
		Reader: rw.Reader,
	}
	r.Write([]byte(http2.ClientPreface))
	w := &writeRecorder{
		Writer: rw.Writer,
	}

	framer := http2.NewFramer(w, r)
	framer.SetMaxReadFrameSize(http2MaxFrameLen)
	framer.SetReuseFrames()
	framer.MaxHeaderListSize = defaultServerMaxHeaderListSize
	framer.ReadMetaHeaders = hpack.NewDecoder(http2InitHeaderTableSize, nil)

	frame, err := framer.ReadFrame()
	if err != nil {
		return nil, err
	}

	_, ok := frame.(*http2.SettingsFrame)
	if !ok {
		return nil, ErrInlvaidGRPCPreface
	}

	isettings := []http2.Setting{{
		ID:  http2.SettingMaxFrameSize,
		Val: http2MaxFrameLen,
	}}
	if err := framer.WriteSettings(isettings...); err != nil {
		return nil, err
	}
	if err := framer.WriteSettingsAck(); err != nil {
		return nil, err
	}

	if err := rw.Flush(); err != nil {
		return nil, err
	}

	frame, err = framer.ReadFrame()
	if err != nil {
		return nil, err
	}
	_, ok = frame.(*http2.SettingsFrame)
	if !ok {
		return nil, ErrInlvaidGRPCPreface
	}

	frame, err = framer.ReadFrame()
	if err != nil {
		return nil, err
	}

	metaHeader, ok := frame.(*http2.MetaHeadersFrame)
	if !ok {
		return nil, ErrInlvaidGRPCPreface
	}

	return &GRPCPreface{
		Header:        metaHeader.Fields,
		ServerPreface: w.Bytes(),
		ClientPreface: r.Bytes(),
	}, nil
}

type grpcServer struct {
	serveHTTP func(rw http.ResponseWriter, req *http.Request)
}

func (s *grpcServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	s.serveHTTP(rw, req)
}

func (resolver *HttpPortForwardResolver) GetGRPCHandler(base http.Handler, client *rest.RESTClient, config *rest.Config, namespace string) http.Handler {
	handleConnection := func(local net.Conn, preface *GRPCPreface) error {
		addr := ""

		// see https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md#requests
		for _, f := range preface.Header {
			if f.Name != ":authority" {
				continue
			}
			addr = f.Value
		}

		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return err
		}

		director := resolver.LookupServiceDirector(host)
		if director == nil {
			// TODO http 503
			err = fmt.Errorf("%s svc not found", host)
			runtime.HandleError(err)
			return err
		}

		backend := director.LookupPodBackend()
		if backend == nil {
			err := fmt.Errorf("%s pod not found", director.Name())
			return err
		}

		conn, err := backend.DialPortForwardOnce(client, config, namespace)
		if err != nil {
			director.Evict(backend.Name())
			return err
		}
		conn.OnCreateStream = backend.OnCreateStream
		conn.OnCloseStream = backend.OnCloseStream

		go func() {
			err := conn.Forward(local, director.TargetPort(), preface.ServerPreface, preface.ClientPreface)
			if err != nil {
				director.Evict(backend.Name())
			}
		}()
		return nil
	}
	handler := &grpcServer{
		serveHTTP: func(res http.ResponseWriter, req *http.Request) {
			if req.ProtoMajor == 2 {
				h, ok := res.(http.Hijacker)
				if !ok {
					return
				}

				conn, rw, err := h.Hijack()
				if err != nil {
					return
				}

				preface, err := GetGRPCPreface(conn, rw)
				if err != nil {
					return
				}
				handleConnection(conn, preface)
				return
			}
			base.ServeHTTP(res, req)
		},
	}
	return handler
}
