package entity

type Route struct {
	Name string
	Host string
}

// Equal returns true if two routes have same fields with same values.
func (r *Route) Equal(other *Route) bool {
	if other == nil {
		return false
	}

	if r.Name != other.Name {
		return false
	}

	if r.Host != other.Host {
		return false
	}

	return true
}
