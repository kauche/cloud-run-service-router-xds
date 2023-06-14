package cloudrun

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/kauche/cloud-run-service-router-xds/internal/domain/entity"
)

var (
	_                 runpb.ServicesServer = (*testCloudRunServicesServer)(nil)
	firstPageServices                      = []*runpb.Service{
		{
			Name:       "projects/test-project/locations/test-location/services/origin-service-1",
			Uid:        "8748ff67-5f1c-4df1-b507-9cbb18950a07",
			Uri:        "https://origin-service-1-test-an.a.run.app",
			Generation: 1,
		},
		{
			Name:        "projects/test-project/locations/test-location/services/route-service-1",
			Uid:         "b6c2cda0-dd8c-40ed-af1e-86effe719ffc",
			Uri:         "https://route-service-1-test-an.a.run.app",
			Generation:  1,
			Annotations: map[string]string{originServiceAnnotation: "origin-service-1"},
		},
		{
			Name:       "projects/test-project/locations/test-location/services/origin-service-without-route",
			Uid:        "1742dae4-4dfa-4061-8b90-727f25e5c6dd",
			Uri:        "https://origin-service-without-route-test-an.a.run.app",
			Generation: 1,
		},
		{
			Name:        "projects/test-project/locations/test-location/services/route-service-without-origin",
			Uid:         "1cc14600-5eff-4ed2-9c89-f3c5c6d30ef1",
			Uri:         "https://route-service-without-origin-test-an.a.run.app",
			Generation:  1,
			Annotations: map[string]string{originServiceAnnotation: "route-service-without-origin"},
		},
	}
	secondPageServices = []*runpb.Service{
		{
			Name:       "projects/test-project/locations/test-location/services/origin-service-2",
			Uid:        "b1a2cef0-b570-40b9-8de0-09966912bc0f",
			Generation: 1,
			Uri:        "https://origin-service-2-test-an.a.run.app",
		},
		{
			Name:        "projects/test-project/locations/test-location/services/route-service-2",
			Uid:         "04c21e30-0f9e-401c-bc11-0e920428df27",
			Generation:  1,
			Uri:         "https://route-service-2-test-an.a.run.app",
			Annotations: map[string]string{originServiceAnnotation: "origin-service-2"},
		},
		{
			Name:        "projects/test-project/locations/test-location/services/route-service-3",
			Uid:         "e1760a39-09fd-4f98-b842-a21413c367ca",
			Generation:  1,
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

	repo, err := NewServiceRepository(ctx, "test-project", "test-location", endpoint)
	if err != nil {
		t.Errorf("failed to create the service repository: %s", err)
		return
	}

	if err = repo.RefreshServices(ctx); err != nil {
		t.Errorf("failed to refresh services: %s", err)
		return
	}

	want := []*entity.Service{
		{
			Name:    "origin-service-1",
			Version: "b543a676722f1d45cdd5b7c4b9c4ce939cc14896e0251d36c789c9d812b65a89",
			DefaultRoute: &entity.Route{
				Name:    "origin-service-1",
				Host:    "origin-service-1-test-an.a.run.app",
				Version: "8748ff67-5f1c-4df1-b507-9cbb18950a07-1",
			},
			Routes: map[string]*entity.Route{
				"route-service-1": {
					Name:    "route-service-1",
					Host:    "route-service-1-test-an.a.run.app",
					Version: "b6c2cda0-dd8c-40ed-af1e-86effe719ffc-1",
				},
			},
		},
		{
			Name:    "origin-service-2",
			Version: "9630692759893938f2f580f7f1add146279dab745e4a22f7c972f36c53c608bb",
			DefaultRoute: &entity.Route{
				Name:    "origin-service-2",
				Host:    "origin-service-2-test-an.a.run.app",
				Version: "b1a2cef0-b570-40b9-8de0-09966912bc0f-1",
			},
			Routes: map[string]*entity.Route{
				"route-service-2": {
					Name:    "route-service-2",
					Host:    "route-service-2-test-an.a.run.app",
					Version: "04c21e30-0f9e-401c-bc11-0e920428df27-1",
				},
				"route-service-3": {
					Name:    "route-service-3",
					Host:    "route-service-3-test-an.a.run.app",
					Version: "e1760a39-09fd-4f98-b842-a21413c367ca-1",
				},
			},
		},
		{
			Name:    "origin-service-without-route",
			Version: "a7928920b8b5fdab798afdb07a8a2e3795c0d932f2f42e0e4f34c27895357ffe",
			DefaultRoute: &entity.Route{
				Name:    "origin-service-without-route",
				Host:    "origin-service-without-route-test-an.a.run.app",
				Version: "1742dae4-4dfa-4061-8b90-727f25e5c6dd-1",
			},
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
