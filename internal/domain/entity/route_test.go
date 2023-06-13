package entity

import "testing"

func TestRouteEqual(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		route *Route
		other *Route
		want  bool
	}{
		"should return true if two routes have same fields with same values": {
			route: &Route{
				Name: "test",
				Host: "test.example.com",
			},
			other: &Route{
				Name: "test",
				Host: "test.example.com",
			},
			want: true,
		},
		"should return false if two routes have the different Name": {
			route: &Route{
				Name: "test",
				Host: "test.example.com",
			},
			other: &Route{
				Name: "xxx",
				Host: "test.example.com",
			},
			want: false,
		},
		"should return false if two routes have the different Host": {
			route: &Route{
				Name: "test",
				Host: "test.example.com",
			},
			other: &Route{
				Name: "test",
				Host: "xxx.example.com",
			},
			want: false,
		},
		"should return false if two routes have the different Version": {
			route: &Route{
				Name:    "test",
				Host:    "test.example.com",
				Version: "94ba4b1f-8c68-4dd6-adf0-438539f9f494-1",
			},
			other: &Route{
				Name:    "test",
				Host:    "test.example.com",
				Version: "4eba45be-880d-4973-a1e9-42d093ca0727-1",
			},
			want: false,
		},
		"should return false if the route passed as the argument is nil": {
			route: &Route{
				Name: "test",
				Host: "test.example.com",
			},
			other: nil,
			want:  false,
		},
	} {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := test.route.Equal(test.other); got != test.want {
				t.Errorf("want %v, got %v", test.want, got)
			}
		})
	}
}
