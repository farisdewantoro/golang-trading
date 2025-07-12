package dto

type DataTimeframe struct {
	Interval string `json:"interval"`
	Range    string `json:"range"`
	Weight   int    `json:"weight"`
	IsMain   bool   `json:"is_main"`
}

func (d *DataTimeframe) ToTradingViewScreenersInterval() string {
	switch d.Interval {
	case Interval30Min:
		return TradingViewInterval30Min
	case Interval1Hour:
		return TradingViewInterval1Hour
	case Interval4Hour:
		return TradingViewInterval4Hour
	case Interval1Day:
		return TradingViewInterval1Day
	case Interval1Week:
		return TradingViewInterval1Week
	default:
		return TradingViewInterval1Day
	}
}

func MapTradingViewScreenerRecommend(val int) string {
	switch val {
	case TradingViewSignalStrongBuy:
		return SignalBuy
	case TradingViewSignalBuy:
		return SignalBuy
	case TradingViewSignalNeutral:
		return SignalNeutral
	case TradingViewSignalSell:
		return SignalSell
	case TradingViewSignalStrongSell:
		return SignalStrongSell
	default:
		return "NOT_FOUND"
	}
}
