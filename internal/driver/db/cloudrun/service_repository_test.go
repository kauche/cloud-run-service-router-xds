package cloudrun

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	run "cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/kauche/cloud-run-service-router-xds/internal/domain/entity"
)

var (
	_                 runpb.ServicesServer = (*testCloudRunServicesServer)(nil)
	firstPageServices                      = []*runpb.Service{
		{
			Name: "projects/test-project/locations/test-location/services/origin-service-1",
			Uri:  "https://origin-service-1-test-an.a.run.app",
		},
		{
			Name:        "projects/test-project/locations/test-location/services/route-service-1",
			Uri:         "https://route-service-1-test-an.a.run.app",
			Annotations: map[string]string{originServiceAnnotation: "origin-service-1"},
		},
		{
			Name: "projects/test-project/locations/test-location/services/origin-service-without-route",
			Uri:  "https://origin-service-without-route-test-an.a.run.app",
		},
		{
			Name:        "projects/test-project/locations/test-location/services/route-service-without-origin",
			Uri:         "https://route-service-without-origin-test-an.a.run.app",
			Annotations: map[string]string{originServiceAnnotation: "route-service-without-origin"},
		},
	}
	secondPageServices = []*runpb.Service{
		{
			Name: "projects/test-project/locations/test-location/services/origin-service-2",
			Uri:  "https://origin-service-2-test-an.a.run.app",
		},
		{
			Name:        "projects/test-project/locations/test-location/services/route-service-2",
			Uri:         "https://route-service-2-test-an.a.run.app",
			Annotations: map[string]string{originServiceAnnotation: "origin-service-2"},
		},
		{
			Name:        "projects/test-project/locations/test-location/services/route-service-3",
			Uri:         "https://route-service-3-test-an.a.run.app",
			Annotations: map[string]string{originServiceAnnotation: "origin-service-2"},
		},
	}
)

type testCloudRunServicesServer struct {
	runpb.UnimplementedServicesServer
}

const testNextPageToken = "next-page-token"

func (t *testCloudRunServicesServer) ListServices(ctx context.Context, req *runpb.ListServicesRequest) (*runpb.ListServicesResponse, error) {
	var res *runpb.ListServicesResponse

	if req.PageToken == "" {
		res = &runpb.ListServicesResponse{
			NextPageToken: testNextPageToken,
			Services:      firstPageServices,
		}
	} else {
		res = &runpb.ListServicesResponse{
			NextPageToken: "",
			Services:      secondPageServices,
		}
	}

	return res, nil
}

var endpoint string

func TestMain(m *testing.M) {
	os.Exit(func() int {
		gs := grpc.NewServer()
		runpb.RegisterServicesServer(gs, &testCloudRunServicesServer{})
		reflection.Register(gs)

		termCh := make(chan struct{})

		defer func() {
			gs.GracefulStop()
			<-termCh
		}()

		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to listen on the ephemeral port: %s", err)
			return 1
		}

		endpoint = listener.Addr().String()

		go func() {
			defer func() {
				close(termCh)
			}()

			if err := gs.Serve(listener); err != nil {
				fmt.Fprintf(os.Stderr, "the cloud run server has aborted: %s\n", err)
			}
		}()

		return m.Run()
	}())
}

func TestRefreshServices(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	client, err := run.NewServicesClient(
		ctx,
		option.WithEndpoint(endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		t.Errorf("failed to create the cloud run client: %s", err)
		return
	}

	repo := &ServiceRepository{
		client:   client,
		project:  "test-project",
		location: "test-location",
	}

	if err = repo.RefreshServices(ctx); err != nil {
		t.Errorf("failed to refresh services: %s", err)
		return
	}

	want := []*entity.Service{
		{
			Name:        "origin-service-1",
			DefaultHost: "origin-service-1-test-an.a.run.app",
			Routes: map[string]*entity.Route{
				"route-service-1": {
					Name: "route-service-1",
					Host: "route-service-1-test-an.a.run.app",
				},
			},
		},
		{
			Name:        "origin-service-2",
			DefaultHost: "origin-service-2-test-an.a.run.app",
			Routes: map[string]*entity.Route{
				"route-service-2": {
					Name: "route-service-2",
					Host: "route-service-2-test-an.a.run.app",
				},
				"route-service-3": {
					Name: "route-service-3",
					Host: "route-service-3-test-an.a.run.app",
				},
			},
		},
		{
			Name:        "origin-service-without-route",
			DefaultHost: "origin-service-without-route-test-an.a.run.app",
		},
	}

	got, err := repo.ListAllServices(ctx)
	if err != nil {
		t.Errorf("failed to call ListAllServices: %s", err)
		return
	}

	if diff := cmp.Diff(got, want, cmpopts.SortSlices(func(x, y *entity.Service) bool {
		return strings.Compare(x.Name, y.Name) < 0
	})); diff != "" {
		t.Errorf("\n(-got, +want)\n%s", diff)
	}
}
