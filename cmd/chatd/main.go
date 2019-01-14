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
	
	chat_api_v1 "github.com/jhayotte/chat/api/v1/chatd"
	chat_service_v1 "github.com/jhayotte/chat/service/v1/chat"

	proxy_runtime "github.com/grpc-ecosystem/grpc-gateway/runtime"
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
	bindHTTP    string
	bindGRPC    string
	storagePath string
}

func init() {
	// Logrus
	logger = log.NewEntry(log.New())
	logger = logger.WithFields(log.Fields{
		"error":                "",
		"grpc.code":            "",
		"grpc.method":          "",
		"grpc.request.content": "",
		"grpc.service":         "",
		"grpc.start_time":      "",
		"grpc.time_ms":         "",
		"peer.address":         "",
	})

	grpc_logrus.ReplaceGrpcLogger(logger)
	log.SetLevel(log.InfoLevel)
}

func main() {
	errc := make(chan error)
	defer close(errc)

	// Read configuration from a config file (ip/port/log all message into a log file)
	// TODO...
	s := server{
		bindGRPC: "0.0.0.0:2338",
		bindHTTP: "0.0.0.0:8080",
	}

	lis, err := net.Listen("tcp", s.bindGRPC)
	if err != nil {
		log.Fatalf("Failed to listen: %v", s.bindGRPC)
	}

	// Decider alwaysLoggingDeciderServer.
	alwaysLoggingDeciderServer := func(ctx context.Context, fullMethodName string, servingObject interface{}) bool {
		return true
	}

	var grpcServer *grpc.Server
	grpcServer = grpc.NewServer(grpc.UnaryInterceptor(
		grpc_middleware.ChainUnaryServer(
			grpc_logrus.PayloadUnaryServerInterceptor(logger, alwaysLoggingDeciderServer)
		),
	))
	

	// Create a context for easy cancellation
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	// Gracefully shut down on ctrl-c
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-interrupt
		errc <- errors.New("received signal interrupt")
	}()

	// Register Chat service v1 and HTTP service handler
	chatV1 := chat_service_v1.NewChatService()
	
	chat_api_v1.RegisterCampaignServiceServer(grpcServer, chatV1)

	log.Println("Starting Chat service..")

	go func() {
		log.Println(fmt.Sprintf("Starting GRPC server on %s", s.bindGRPC))
		errc <- errors.Wrap(grpcServer.Serve(lis), "Cannot start GRPC server")
	}()

	conn, err = grpc.Dial(s.bindGRPC, grpc.WithInsecure())
	if err != nil {
		panic("Couldn't contact grpc server")
	}

	// HTTP R-Proxy grpc-gateway
	mux := proxy_runtime.NewServeMux(proxy_runtime.WithMarshalerOption(proxy_runtime.MIMEWildcard, &proxy_runtime.JSONPb{OrigName: true, EmitDefaults: true}))
	err = chat_api_v1.RegisterChatServiceHandler(ctx, mux, conn)
	if err != nil {
		panic("Cannot serve chatd http api v1")
	}
	go func() {
		log.Println(fmt.Sprintf("Starting HTTP server on %s", s.bindHTTP))
		//handlerEnriched := cors(preMuxRouter(mux))
		errc <- errors.Wrap(http.ListenAndServe(s.bindHTTP, mux), "Cannot start HTTP server")
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
