package main

import (
	"context"
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/status"
	"log"
	"net"
	"os"
	"time"

	"github.com/davefinster/uqcs-demo/backend/api"
	"github.com/davefinster/uqcs-demo/backend/store"
	"go.opencensus.io/plugin/ocgrpc"

	"cloud.google.com/go/logging"
	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

var (
	port = flag.Int("port", 10000, "The server port")
)

type server struct {
	store *store.Postgres
	api.UnimplementedEventBackendServer
	log *logging.Logger
}

func (s *server) doLog(payload interface{}, sev logging.Severity, span *trace.Span) {
	s.log.Log(logging.Entry{
		Payload:  payload,
		Severity: sev,
		SpanID:   span.SpanContext().SpanID.String(),
		Trace:    fmt.Sprintf("projects/%s/traces/%s", os.Getenv("GCP_PROJECT_ID"), span.SpanContext().TraceID.String()),
	})
}

func (s *server) GetEvents(ctx context.Context, req *api.GetEventsRequest) (*api.GetEventsResponse, error) {
	ctx, span := trace.StartSpan(ctx, "uqcs.backend.grpc.GetEvents")
	defer span.End()
	events, err := s.store.FetchEvents(ctx, nil)
	if err != nil {
		s.doLog(map[string]interface{}{
			"message": fmt.Sprintf("Error fetching events: %s", err),
			"error":   err,
		}, logging.Error, span)
		return nil, err
	}
	span.Annotate([]trace.Attribute{
		trace.Int64Attribute("fetched_events", int64(len(events))),
		trace.StringAttribute("store", "postgres"),
	}, "Fetch Events from Store")
	s.doLog(fmt.Sprintf("Fetched %d events", len(events)), logging.Info, span)
	return &api.GetEventsResponse{
		Events: events,
	}, nil
}

func (s *server) CreateEvent(ctx context.Context, req *api.CreateEventRequest) (*api.CreateEventResponse, error) {
	ctx, span := trace.StartSpan(ctx, "uqcs.backend.grpc.CreateEvent")
	defer span.End()
	if req.GetEvent() == nil {
		return nil, status.New(codes.InvalidArgument, "an event must be specified").Err()
	}
	if len(req.GetEvent().GetTitle()) == 0 {
		return nil, status.New(codes.InvalidArgument, "an event must have a non-empty title").Err()
	}
	newEvent, err := s.store.CreateEvent(ctx, req.Event)
	if err != nil {
		errText := fmt.Sprintf("Error creating event: %s", err)
		s.doLog(map[string]interface{}{
			"message": errText,
			"error":   err,
		}, logging.Error, span)
		return nil, status.New(codes.Internal, errText).Err()
	}
	span.Annotate([]trace.Attribute{
		trace.StringAttribute("assigned_id", newEvent.GetId()),
		trace.StringAttribute("store", "postgres"),
	}, "Assigned Event ID")
	s.doLog(fmt.Sprintf("Created event with ID %s", newEvent.GetId()), logging.Info, span)
	return &api.CreateEventResponse{
		Event: newEvent,
	}, nil
}

func main() {
	flag.Parse()
	ctx := context.Background()
	project := os.Getenv("GCP_PROJECT_ID")
	exporter, err := stackdriver.NewExporter(stackdriver.Options{
		ProjectID:         project,
		MetricPrefix:      "uqcs",
		ReportingInterval: 10 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer exporter.Flush()
	trace.RegisterExporter(exporter)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	if err := view.Register(ocgrpc.DefaultServerViews...); err != nil {
		log.Fatalf("Failed to register ocgrpc server views: %v", err)
	}
	if err := exporter.StartMetricsExporter(); err != nil {
		log.Fatalf("Error starting metric exporter: %v", err)
	}
	defer exporter.StopMetricsExporter()

	loggingClient, err := logging.NewClient(ctx, project)
	if err != nil {
		log.Fatal(err)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	healthServer := health.NewServer()
	grpcServer := grpc.NewServer(grpc.StatsHandler(&ocgrpc.ServerHandler{}))
	var s *store.Postgres
	for i := 0; i < 10; i++ {
		log.Printf("Attempting to connect to Postgres - Attempt %d\n", i)
		store, err := store.NewPostgres(os.Getenv("CONNECTION_PARAMS"))
		if err != nil {
			time.Sleep(10 * time.Second)
		} else {
			s = store
			break
		}
	}
	if s == nil {
		log.Fatalf("failed to setup store - last error: %v", err)
	}
	lg := loggingClient.Logger("backend")
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	api.RegisterEventBackendServer(grpcServer, &server{
		store: s,
		log:   lg,
	})
	fmt.Printf("Starting to listen")
	healthServer.SetServingStatus("backend", healthpb.HealthCheckResponse_SERVING)
	grpcServer.Serve(lis)
}
