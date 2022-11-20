package entity

type Service struct {
	Name        string
	DefaultHost string
	Version     string
	Routes      map[string]*Route
}

// Equal returns true if two routes have same fields (including Routes) with same values except Version.
func (s *Service) Equal(other *Service) bool {
	if other == nil {
		return false
	}

	if s.Name != other.Name {
		return false
	}

	if s.DefaultHost != other.DefaultHost {
		return false
	}

	for k, v := range s.Routes {
		r, ok := other.Routes[k]
		if !ok {
			return false
		}

		if !v.Equal(r) {
			return false
		}
	}

	return len(s.Routes) == len(other.Routes)
}
