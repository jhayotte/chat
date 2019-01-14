package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime"

	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	proxy_runtime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	chatd_v1 "github.com/jhayotte/chat/api/v1/chatd"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// Panic handler prints the stack trace when recovering from a panic.
	panicHandler = grpc_recovery.RecoveryHandlerFunc(func(p interface{}) error {
		buf := make([]byte, 1<<16)
		runtime.Stack(buf, true)
		logger.Errorf("panic recovered: %+v", string(buf))
		return status.Errorf(codes.Internal, "%s", p)
	})
	logger *log.Entry
	conn   *grpc.ClientConn
	// Default value for grpc max message size (in bytes). Thus 128Mb.
	grpcMaxMessageSize = 128 << 20
)

type server struct {
	port        int
	ip          string
	storagePath string
}

func init() {
	// Logrus
	logger = log.NewEntry(log.New())
	logger = logger.WithFields(log.Fields{
		"error":                 "",
		"grpc.code":             "",
		"grpc.method":           "",
		"grpc.request.content":  "",
		"grpc.response.content": "",
		"grpc.service":          "",
		"grpc.start_time":       "",
		"grpc.time_ms":          "",
		"peer.address":          "",
		"request_id":            "",
		"span.kind":             "",
		"system":                "",
	})

	grpc_logrus.ReplaceGrpcLogger(logger)
	log.SetLevel(log.InfoLevel)
}

func main() {
	errc := make(chan error)
	defer close(errc)

	lis, err := net.Listen("tcp", "0.0.0.0:2338")
	if err != nil {
		log.Fatalf("Failed to listen: %v", "0.0.0.0:2338")
	}

	// Read configuration from a config file (ip/port/log all message into a log file)
	// TODO...

	// Decider alwaysLoggingDeciderServer.
	alwaysLoggingDeciderServer := func(ctx context.Context, fullMethodName string, servingObject interface{}) bool {
		return true
	}

	var grpcServer *grpc.Server
	grpcServer = grpc.NewServer(grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(grpc_logrus.PayloadUnaryServerInterceptor(logger, alwaysLoggingDeciderServer))))

	// Create a context for easy cancellation
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	// HTTP R-Proxy grpc-gateway
	mux := proxy_runtime.NewServeMux(proxy_runtime.WithMarshalerOption(proxy_runtime.MIMEWildcard, &proxy_runtime.JSONPb{OrigName: true, EmitDefaults: true}))
	err = chatd_v1.RegisterCampaignServiceHandler(ctx, mux, conn)
	if err != nil {
		panic("Cannot serve chatd http api v1")
	}
	go func() {
		log.Println(fmt.Sprintf("Starting HTTP server on %s", "0.0.0.0:8080"))
		//handlerEnriched := cors(preMuxRouter(mux))
		errc <- errors.Wrap(http.ListenAndServe("0.0.0.0:8080", mux), "Cannot start HTTP server")
	}()

	fatalError := <-errc
	grpcServer.GracefulStop()
	cancelFunc()
	fmt.Println("Server running")
	if err := lis.Close(); err != nil {
		log.Warning(err)
	}
	log.Println(fatalError.Error())
}
