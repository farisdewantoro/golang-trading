package telegram

const (
	UserStateKey = "user_state:%d"

	StateIdle = iota // 0

	// /setposition states
	StateWaitingSetPositionSymbol       = 1
	StateWaitingSetPositionBuyPrice     = 2
	StateWaitingSetPositionBuyDate      = 3
	StateWaitingSetPositionTakeProfit   = 4
	StateWaitingSetPositionStopLoss     = 5
	StateWaitingSetPositionMaxHolding   = 6
	StateWaitingSetPositionAlertPrice   = 7
	StateWaitingSetPositionAlertMonitor = 8

	// /analyze position states
	StateWaitingAnalysisPositionSymbol   = 10
	StateWaitingAnalysisPositionBuyPrice = 11
	StateWaitingAnalysisPositionBuyDate  = 12
	StateWaitingAnalysisPositionMaxDays  = 13
	StateWaitingAnalysisPositionInterval = 14
	StateWaitingAnalysisPositionPeriod   = 15

	// /analyze main flow states
	StateWaitingAnalyzeSymbol = 20
	StateWaitingAnalysisType  = 21

	// exit position states
	StateWaitingExitPositionInputExitPrice = 30
	StateWaitingExitPositionInputExitDate  = 31
	StateWaitingExitPositionConfirm        = 32

	StateWaitingNewsFindSymbol                  = 40
	StateWaitingNewsFindSendSummaryConfirmation = 41

	StateWaitingAdjustTargetPositionInputTargetPrice   = 50
	StateWaitingAdjustTargetPositionInputStopLossPrice = 51
	StateWaitingAdjustTargetPositionMaxHoldingDays     = 52
	StateWaitingAdjustTargetPositionConfirm            = 53
)
