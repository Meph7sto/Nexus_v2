package service

func (s *GatewayService) SetUsageInteractionService(interactions *UsageInteractionService) {
	if s != nil {
		s.usageInteractionService = interactions
	}
}

func (s *OpenAIGatewayService) SetUsageInteractionService(interactions *UsageInteractionService) {
	if s != nil {
		s.usageInteractionService = interactions
	}
}
