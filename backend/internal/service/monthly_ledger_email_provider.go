package service

func ProvideMonthlyLedgerService(repo MonthlyLedgerRepository, email *NotificationEmailService, settings SettingRepository) *MonthlyLedgerService {
	s := NewMonthlyLedgerService(repo)
	s.emailSettings = settings
	s.emailSend = email.Send
	s.StartEmailNotifications(email)
	s.StartIncomeSchedules()
	return s
}
