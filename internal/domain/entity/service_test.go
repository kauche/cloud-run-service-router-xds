package entity

import "testing"

func TestServiceRoute(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		service *Service
		other   *Service
		want    bool
	}{
		"should return true if two services have same fields with same values": {
			service: &Service{
				Name: "test",
				DefaultRoute: &Route{
					Name:    "test",
					Host:    "test.example.com",
					Version: "94ba4b1f-8c68-4dd6-adf0-438539f9f494-1",
				},
				Version: "4a6e7aa3-a8d3-40e2-97ec-0bef9b85701d",
				Routes: map[string]*Route{
					"test-1": {
						Name: "test-1",
						Host: "test-1.example.com",
					},
					"test-2": {
						Name: "test-2",
						Host: "test-2.example.com",
					},
				},
			},
			other: &Service{
				Name: "test",
				DefaultRoute: &Route{
					Name:    "test",
					Host:    "test.example.com",
					Version: "94ba4b1f-8c68-4dd6-adf0-438539f9f494-1",
				},
				Version: "4a6e7aa3-a8d3-40e2-97ec-0bef9b85701d",
				Routes: map[string]*Route{
					"test-1": {
						Name: "test-1",
						Host: "test-1.example.com",
					},
					"test-2": {
						Name: "test-2",
						Host: "test-2.example.com",
					},
				},
			},
			want: true,
		},
		"should return false if two services have different Version": {
			service: &Service{
				Name: "test",
				DefaultRoute: &Route{
					Name:    "test",
					Host:    "test.example.com",
					Version: "94ba4b1f-8c68-4dd6-adf0-438539f9f494-1",
				},
				Version: "4a6e7aa3-a8d3-40e2-97ec-0bef9b85701d",
				Routes: map[string]*Route{
					"test-1": {
						Name: "test-1",
						Host: "test-1.example.com",
					},
					"test-2": {
						Name: "test-2",
						Host: "test-2.example.com",
					},
				},
			},
			other: &Service{
				Name: "test",
				DefaultRoute: &Route{
					Name:    "test",
					Host:    "test.example.com",
					Version: "94ba4b1f-8c68-4dd6-adf0-438539f9f494-1",
				},
				Version: "341d6116-8b17-4813-bdee-c5667073ca25",
				Routes: map[string]*Route{
					"test-1": {
						Name: "test-1",
						Host: "test-1.example.com",
					},
					"test-2": {
						Name: "test-2",
						Host: "test-2.example.com",
					},
				},
			},
			want: false,
		},
		"should return false if two services have the different Name": {
			service: &Service{
				Name: "test",
				DefaultRoute: &Route{
					Name:    "test",
					Host:    "test.example.com",
					Version: "94ba4b1f-8c68-4dd6-adf0-438539f9f494-1",
				},
				Version: "4a6e7aa3-a8d3-40e2-97ec-0bef9b85701d",
				Routes: map[string]*Route{
					"test-1": {
						Name: "test-1",
						Host: "test-1.example.com",
					},
					"test-2": {
						Name: "test-2",
						Host: "test-2.example.com",
					},
				},
			},
			other: &Service{
				Name: "xxx",
				DefaultRoute: &Route{
					Name:    "test",
					Host:    "test.example.com",
					Version: "94ba4b1f-8c68-4dd6-adf0-438539f9f494-1",
				},
				Version: "4a6e7aa3-a8d3-40e2-97ec-0bef9b85701d",
				Routes: map[string]*Route{
					"test-1": {
						Name: "test-1",
						Host: "test-1.example.com",
					},
					"test-2": {
						Name: "test-2",
						Host: "test-2.example.com",
					},
				},
			},
			want: false,
		},
		"should return false if two services have the different DefaultRoute": {
			service: &Service{
				Name: "test",
				DefaultRoute: &Route{
					Name:    "test",
					Host:    "test.example.com",
					Version: "94ba4b1f-8c68-4dd6-adf0-438539f9f494-1",
				},
				Version: "4a6e7aa3-a8d3-40e2-97ec-0bef9b85701d",
				Routes: map[string]*Route{
					"test-1": {
						Name: "test-1",
						Host: "test-1.example.com",
					},
					"test-2": {
						Name: "test-2",
						Host: "test-2.example.com",
					},
				},
			},
			other: &Service{
				Name: "test",
				DefaultRoute: &Route{
					Name:    "test",
					Host:    "xxx.example.com",
					Version: "21a0afc3-d070-4f0f-a0e7-68dda21515d4-1",
				},
				Version: "4a6e7aa3-a8d3-40e2-97ec-0bef9b85701d",
				Routes: map[string]*Route{
					"test-1": {
						Name: "test-1",
						Host: "test-1.example.com",
					},
					"test-2": {
						Name: "test-2",
						Host: "test-2.example.com",
					},
				},
			},
			want: false,
		},
		"should return false if the service passed as the argument does not have a Route that exists in caller's Routes": {
			service: &Service{
				Name: "test",
				DefaultRoute: &Route{
					Name:    "test",
					Host:    "test.example.com",
					Version: "94ba4b1f-8c68-4dd6-adf0-438539f9f494-1",
				},
				Version: "4a6e7aa3-a8d3-40e2-97ec-0bef9b85701d",
				Routes: map[string]*Route{
					"test-1": {
						Name: "test-1",
						Host: "test-1.example.com",
					},
					"test-2": {
						Name: "test-2",
						Host: "test-2.example.com",
					},
				},
			},
			other: &Service{
				Name: "test",
				DefaultRoute: &Route{
					Name:    "test",
					Host:    "test.example.com",
					Version: "94ba4b1f-8c68-4dd6-adf0-438539f9f494-1",
				},
				Version: "4a6e7aa3-a8d3-40e2-97ec-0bef9b85701d",
				Routes: map[string]*Route{
					"test-1": {
						Name: "test-1",
						Host: "test-1.example.com",
					},
				},
			},
			want: false,
		},
		"should return false if the service passed as the argument has a Route that does not exists in caller's Routes": {
			service: &Service{
				Name: "test",
				DefaultRoute: &Route{
					Name:    "test",
					Host:    "test.example.com",
					Version: "94ba4b1f-8c68-4dd6-adf0-438539f9f494-1",
				},
				Version: "4a6e7aa3-a8d3-40e2-97ec-0bef9b85701d",
				Routes: map[string]*Route{
					"test-1": {
						Name: "test-1",
						Host: "test-1.example.com",
					},
					"test-2": {
						Name: "test-2",
						Host: "test-2.example.com",
					},
				},
			},
			other: &Service{
				Name: "test",
				DefaultRoute: &Route{
					Name:    "test",
					Host:    "test.example.com",
					Version: "94ba4b1f-8c68-4dd6-adf0-438539f9f494-1",
				},
				Version: "4a6e7aa3-a8d3-40e2-97ec-0bef9b85701d",
				Routes: map[string]*Route{
					"test-1": {
						Name: "test-1",
						Host: "test-1.example.com",
					},
					"test-3": {
						Name: "test-3",
						Host: "test-3.example.com",
					},
				},
			},
			want: false,
		},
		"should return false if the service passed as the argument has a Route that does exists in caller's Routes but has a different field": {
			service: &Service{
				Name: "test",
				DefaultRoute: &Route{
					Name:    "test",
					Host:    "test.example.com",
					Version: "94ba4b1f-8c68-4dd6-adf0-438539f9f494-1",
				},
				Version: "4a6e7aa3-a8d3-40e2-97ec-0bef9b85701d",
				Routes: map[string]*Route{
					"test-1": {
						Name: "test-1",
						Host: "test-1.example.com",
					},
					"test-2": {
						Name: "test-2",
						Host: "test-2.example.com",
					},
				},
			},
			other: &Service{
				Name: "test",
				DefaultRoute: &Route{
					Name:    "test",
					Host:    "test.example.com",
					Version: "94ba4b1f-8c68-4dd6-adf0-438539f9f494-1",
				},
				Version: "4a6e7aa3-a8d3-40e2-97ec-0bef9b85701d",
				Routes: map[string]*Route{
					"test-1": {
						Name: "test-1",
						Host: "test-1.example.com",
					},
					"test-2": {
						Name: "test-2",
						Host: "xxx.example.com", // different host
					},
				},
			},
			want: false,
		},
		"should return false if the service passed as the argument is nil": {
			service: &Service{
				Name: "test",
				DefaultRoute: &Route{
					Name:    "test",
					Host:    "test.example.com",
					Version: "94ba4b1f-8c68-4dd6-adf0-438539f9f494-1",
				},
				Version: "4a6e7aa3-a8d3-40e2-97ec-0bef9b85701d",
				Routes: map[string]*Route{
					"test-1": {
						Name: "test-1",
						Host: "test-1.example.com",
					},
					"test-2": {
						Name: "test-2",
						Host: "test-2.example.com",
					},
				},
			},
			other: nil,
			want:  false,
		},
	} {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := test.service.Equal(test.other); got != test.want {
				t.Errorf("want %v, got %v", test.want, got)
			}
		})
	}
}
