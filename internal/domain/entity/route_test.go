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
