package system

type Service struct{}

// NewService constructs a system metrics service.
func NewService() *Service {
	return &Service{}
}

func (s *Service) GetStatus() *SystemStats {
	return GetSystemStats()
}
